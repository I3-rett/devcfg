package steps

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/resolver"
	"github.com/I3-rett/devcfg/internal/system"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type installResultMsg struct {
	name   string
	output string
	err    error
}

type uninstallResultMsg struct {
	name   string
	output string
	err    error
}

type toolDetectMsg struct {
	versions []string // one entry per tool; empty string means not installed
}

// toolItem is one row in the tree-shaped display list.
type toolItem struct {
	idx             int    // index in ToolsModel.tools
	depth           int    // 0 = root, 1 = first-level child, ...
	isLast          bool   // last sibling at this depth level
	parentContinues []bool // for each ancestor at depth >= 1, whether it had more siblings
}

// treePrefix builds the visual tree connector string for a toolItem.
// depth=0 -> empty; depth=1 -> "├── " or "└── "; depth=2 -> "│   ├── " etc.
func treePrefix(item toolItem) string {
	if item.depth == 0 {
		return ""
	}
	var sb strings.Builder
	for _, cont := range item.parentContinues {
		if cont {
			sb.WriteString("│   ")
		} else {
			sb.WriteString("    ")
		}
	}
	if item.isLast {
		sb.WriteString("└── ")
	} else {
		sb.WriteString("├── ")
	}
	return sb.String()
}

// buildDisplayOrder returns tools in tree order: each tool's dependents appear
// directly below it, indented. Tools without requires (or whose requires are
// not in the registry) are roots. A visited set guards against cycles and
// duplicate entries so a malformed tools.json cannot cause infinite recursion.
func buildDisplayOrder(tools []registry.Tool) []toolItem {
	nameToIdx := make(map[string]int, len(tools))
	for i, t := range tools {
		nameToIdx[t.Name] = i
	}

	childrenOf := make(map[string][]int)
	for i, t := range tools {
		for _, req := range t.Requires {
			if _, exists := nameToIdx[req]; exists {
				childrenOf[req] = append(childrenOf[req], i)
			}
		}
	}

	isChild := make([]bool, len(tools))
	for _, children := range childrenOf {
		for _, idx := range children {
			isChild[idx] = true
		}
	}

	roots := make([]int, 0, len(tools))
	for i := range tools {
		if !isChild[i] {
			roots = append(roots, i)
		}
	}

	visited := make([]bool, len(tools))
	var order []toolItem
	var addItem func(idx, depth int, isLast bool, parentContinues []bool)
	addItem = func(idx, depth int, isLast bool, parentContinues []bool) {
		if visited[idx] {
			return
		}
		visited[idx] = true
		order = append(order, toolItem{
			idx:             idx,
			depth:           depth,
			isLast:          isLast,
			parentContinues: parentContinues,
		})
		children := childrenOf[tools[idx].Name]
		for j, childIdx := range children {
			childIsLast := j == len(children)-1
			// parentContinues for depth-1 child: ancestors at depth>=1 only.
			// A root (depth=0) does not contribute a continuation line.
			var childParentCont []bool
			if depth > 0 {
				childParentCont = make([]bool, len(parentContinues)+1)
				copy(childParentCont, parentContinues)
				childParentCont[len(parentContinues)] = !isLast
			}
			addItem(childIdx, depth+1, childIsLast, childParentCont)
		}
	}

	for j, rootIdx := range roots {
		addItem(rootIdx, 0, j == len(roots)-1, nil)
	}
	return order
}

// ToolsModel is the Bubble Tea model for the tool-selection step.
type ToolsModel struct {
	tools        []registry.Tool
	nameToIdx    map[string]int // tool name -> index in tools
	displayOrder []toolItem     // tree-ordered display items
	checked      []bool         // indexed by tool index
	versions     []string       // indexed by tool index; empty = not installed
	toUninstall  []bool         // indexed by tool index; installed tools confirmed for removal
	loaded       bool
	cursor       int // position in displayOrder (0 ... len(displayOrder) = Continue)
	sysInfo      system.Info
	done         bool
	running      bool
	operationCount int // total number of operations (installs + uninstalls)
	results      []string
	errors       []string
	msgLines     []string

	// confirmation popup state
	popupMode          bool     // removal confirmation popup is currently shown
	popupToolDP        int      // display position of the tool awaiting confirmation
	popupDeps          []string // names of INSTALLED checked tools that will also be uninstalled
	popupDeselectedDeps []string // names of checked-but-not-installed tools that will be deselected
	popupCursor        int      // 0 = "Yes, Remove"; 1 = "Cancel"
}

// NewToolsModel initialises the model with the full tool registry.
func NewToolsModel(sysInfo system.Info) *ToolsModel {
	tools := registry.List()
	nameToIdx := make(map[string]int, len(tools))
	for i, t := range tools {
		nameToIdx[t.Name] = i
	}
	return &ToolsModel{
		tools:        tools,
		nameToIdx:    nameToIdx,
		displayOrder: buildDisplayOrder(tools),
		checked:      make([]bool, len(tools)),
		versions:     make([]string, len(tools)),
		toUninstall:  make([]bool, len(tools)),
		sysInfo:      sysInfo,
	}
}

func (m *ToolsModel) Title() string { return "Tools Installation" }
func (m *ToolsModel) IsDone() bool  { return m.done }

func (m *ToolsModel) Init() tea.Cmd {
	tools := m.tools
	return func() tea.Msg {
		versions := make([]string, len(tools))
		for i, t := range tools {
			versions[i] = system.DetectToolVersion(t.BinaryName())
		}
		return toolDetectMsg{versions: versions}
	}
}

// isAvailable returns true when all tools listed in Requires for the item at
// displayPos are either already installed (version detected) or checked, AND
// none of those required tools are pending uninstallation.
func (m *ToolsModel) isAvailable(displayPos int) bool {
	tool := m.tools[m.displayOrder[displayPos].idx]
	for _, req := range tool.Requires {
		reqIdx, ok := m.nameToIdx[req]
		if !ok {
			continue
		}
		if m.toUninstall[reqIdx] || (m.versions[reqIdx] == "" && !m.checked[reqIdx]) {
			return false
		}
	}
	return true
}

// cascadeUncheck iteratively unchecks every checked tool that has become
// unavailable. This propagates transitively through the dependency tree.
func (m *ToolsModel) cascadeUncheck() {
	changed := true
	for changed {
		changed = false
		for dp, item := range m.displayOrder {
			if m.checked[item.idx] && !m.isAvailable(dp) {
				m.checked[item.idx] = false
				changed = true
			}
		}
	}
}

// setChecked updates the checked state for the tool at displayPos and
// propagates auto-unchecks to any dependents that would become unavailable.
// When re-checking a tool that was pending removal, the removal is cancelled.
func (m *ToolsModel) setChecked(displayPos int, val bool) {
	idx := m.displayOrder[displayPos].idx
	m.checked[idx] = val
	if val {
		m.toUninstall[idx] = false // cancel any pending uninstall
	}
	if !val {
		m.cascadeUncheck()
	}
}

// checkedDependentsOf returns the names of all currently-checked tools that
// transitively depend on the tool at toolIdx.
func (m *ToolsModel) checkedDependentsOf(toolIdx int) []string {
	var names []string
	visited := make(map[int]bool)
	var walk func(idx int)
	walk = func(idx int) {
		name := m.tools[idx].Name
		for i, t := range m.tools {
			if visited[i] {
				continue
			}
			for _, req := range t.Requires {
				if req == name {
					visited[i] = true
					if m.checked[i] {
						names = append(names, t.Name)
					}
					walk(i)
					break
				}
			}
		}
	}
	walk(toolIdx)
	return names
}

// applyConfirmedRemoval unchecks the tool at dp and any checked tools that
// transitively depend on it, marking installed ones for uninstallation.
func (m *ToolsModel) applyConfirmedRemoval(dp int) {
	idx := m.displayOrder[dp].idx
	m.checked[idx] = false
	if m.versions[idx] != "" {
		m.toUninstall[idx] = true
	}
	// Cascade: uncheck dependents whose required tool is now pending removal.
	// We use a stricter availability check that treats toUninstall deps as gone.
	changed := true
	for changed {
		changed = false
		for dp2, item2 := range m.displayOrder {
			if !m.checked[item2.idx] {
				continue
			}
			if m.hasRemovalPendingDep(dp2) {
				m.checked[item2.idx] = false
				if m.versions[item2.idx] != "" {
					m.toUninstall[item2.idx] = true
				}
				changed = true
			}
		}
	}
}

// hasRemovalPendingDep returns true when any requirement of the tool at
// displayPos is either being removed (toUninstall) or no longer available.
func (m *ToolsModel) hasRemovalPendingDep(displayPos int) bool {
	return !m.isAvailable(displayPos)
}

func (m *ToolsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle popup confirmation before anything else.
	if m.popupMode {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "left", "h":
				m.popupCursor = 0
			case "right", "l":
				m.popupCursor = 1
			case " ", "enter":
				if m.popupCursor == 0 {
					m.applyConfirmedRemoval(m.popupToolDP)
				}
				m.popupMode = false
			case "esc":
				m.popupMode = false
			}
		}
		return m, nil
	}

	if m.running {
		switch msg := msg.(type) {
		case installResultMsg:
			if msg.err != nil {
				m.errors = append(m.errors, fmt.Sprintf("✗ %s: %s", msg.name, msg.err.Error()))
			} else {
				m.results = append(m.results, fmt.Sprintf("✓ %s installed", msg.name))
			}
			m.msgLines = append(m.msgLines, msg.output)
			if len(m.results)+len(m.errors) >= m.operationCount {
				m.running = false
				m.done = true
			}
		case uninstallResultMsg:
			if msg.err != nil {
				m.errors = append(m.errors, fmt.Sprintf("✗ %s: %s", msg.name, msg.err.Error()))
			} else {
				m.results = append(m.results, fmt.Sprintf("✓ %s removed", msg.name))
			}
			m.msgLines = append(m.msgLines, msg.output)
			if len(m.results)+len(m.errors) >= m.operationCount {
				m.running = false
				m.done = true
			}
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case toolDetectMsg:
		n := len(m.tools)
		if len(msg.versions) < n {
			n = len(msg.versions)
		}
		for i := 0; i < n; i++ {
			m.versions[i] = msg.versions[i]
			if msg.versions[i] != "" {
				m.checked[i] = true
			}
		}
		// Uncheck tools whose dependencies are not yet installed/selected.
		m.cascadeUncheck()
		m.loaded = true
		return m, nil
	}

	if !m.loaded {
		return m, nil
	}

	continueIdx := len(m.displayOrder)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < continueIdx {
				m.cursor++
			}
		case " ":
			if m.cursor < continueIdx && m.isAvailable(m.cursor) {
				m.toggleOrConfirm(m.cursor)
			}
		case "enter":
			if m.cursor < continueIdx {
				if m.isAvailable(m.cursor) {
					m.toggleOrConfirm(m.cursor)
				}
			} else {
				return m, m.startInstallation()
			}
		}
	}
	return m, nil
}

// toggleOrConfirm either directly toggles the checked state of the tool at
// displayPos, or opens the removal confirmation popup when the tool is
// currently checked and already installed.
func (m *ToolsModel) toggleOrConfirm(displayPos int) {
	idx := m.displayOrder[displayPos].idx
	if m.checked[idx] && m.versions[idx] != "" {
		// Tool is installed — ask before removing.
		// Split dependents into installed (will be uninstalled) vs. selected-only (will be deselected).
		var installedDeps, deselectedDeps []string
		for _, name := range m.checkedDependentsOf(idx) {
			if depIdx, ok := m.nameToIdx[name]; ok && m.versions[depIdx] != "" {
				installedDeps = append(installedDeps, name)
			} else {
				deselectedDeps = append(deselectedDeps, name)
			}
		}
		m.popupToolDP = displayPos
		m.popupDeps = installedDeps
		m.popupDeselectedDeps = deselectedDeps
		m.popupCursor = 0
		m.popupMode = true
	} else {
		m.setChecked(displayPos, !m.checked[idx])
	}
}

func (m *ToolsModel) startInstallation() tea.Cmd {
	// Collect tools to uninstall in reverse display order (dependents before parents).
	var toUninstallTools []registry.Tool
	for i := len(m.displayOrder) - 1; i >= 0; i-- {
		item := m.displayOrder[i]
		if m.toUninstall[item.idx] {
			toUninstallTools = append(toUninstallTools, m.tools[item.idx])
		}
	}

	// Collect tools to install (checked but not yet installed) in display order
	// so that dependencies are installed before their dependents.
	var toInstallTools []registry.Tool
	for dp, item := range m.displayOrder {
		if m.checked[item.idx] && m.versions[item.idx] == "" && m.isAvailable(dp) {
			toInstallTools = append(toInstallTools, m.tools[item.idx])
		}
	}

	total := len(toUninstallTools) + len(toInstallTools)
	if total == 0 {
		m.done = true
		return nil
	}
	m.running = true
	m.operationCount = total

	cmds := make([]tea.Cmd, 0, total)
	for _, tool := range toUninstallTools {
		t := tool
		cmds = append(cmds, func() tea.Msg {
			sysInfo := system.Detect()
			args, err := resolver.ResolveUninstall(t, sysInfo)
			if err != nil {
				return uninstallResultMsg{name: t.Name, err: err}
			}
			res := executor.Execute(args)
			return uninstallResultMsg{name: t.Name, output: res.Output, err: res.Err}
		})
	}
	for _, tool := range toInstallTools {
		t := tool
		cmds = append(cmds, func() tea.Msg {
			// Re-detect the package manager at install time so that a dependency
			// installed earlier in the sequence (e.g. brew) is visible to the
			// resolver when processing subsequent tools.
			sysInfo := system.Detect()
			args, err := resolver.Resolve(t, sysInfo)
			if err != nil {
				return installResultMsg{name: t.Name, err: err}
			}
			res := executor.Execute(args)
			return installResultMsg{name: t.Name, output: res.Output, err: res.Err}
		})
	}
	return tea.Sequence(cmds...)
}

func (m *ToolsModel) View() string {
	var sb strings.Builder

	if m.popupMode {
		return m.viewPopup()
	}

	if m.running {
		sb.WriteString(tuistyles.StatusStyle.Render("Applying changes...") + "\n\n")
		for _, r := range m.results {
			sb.WriteString(tuistyles.SuccessStyle.Render(r) + "\n")
		}
		for _, e := range m.errors {
			sb.WriteString(tuistyles.ErrorStyle.Render(e) + "\n")
		}
		return sb.String()
	}

	if m.done {
		sb.WriteString(tuistyles.SuccessStyle.Render("Done!") + "\n\n")
		for _, r := range m.results {
			sb.WriteString(tuistyles.SuccessStyle.Render(r) + "\n")
		}
		for _, e := range m.errors {
			sb.WriteString(tuistyles.ErrorStyle.Render(e) + "\n")
		}
		return sb.String()
	}

	if !m.loaded {
		sb.WriteString(tuistyles.StatusStyle.Render("Detecting installed tools...") + "\n")
		return sb.String()
	}

	sb.WriteString(tuistyles.StatusStyle.Render("Select tools to install/remove (SPACE/ENTER to toggle):") + "\n\n")

	for dp, item := range m.displayOrder {
		tool := m.tools[item.idx]
		available := m.isAvailable(dp)
		checked := m.checked[item.idx]
		pendingRemoval := m.toUninstall[item.idx]

		cursorStr := "  "
		if m.cursor == dp {
			cursorStr = tuistyles.SelectedItemStyle.Render("▶ ")
		}

		var checkbox string
		switch {
		case pendingRemoval:
			checkbox = tuistyles.ErrorStyle.Render("[✗]")
		case checked:
			checkbox = tuistyles.CheckedItemStyle.Render("[✓]")
		default:
			checkbox = "[ ]"
		}

		prefix := treePrefix(item)
		nameDesc := fmt.Sprintf("%-12s %s", tool.Name, tool.Description)

		var rowContent string
		if !available {
			hint := " [requires: " + strings.Join(tool.Requires, ", ") + "]"
			rowContent = tuistyles.DisabledItemStyle.Render(prefix + "[  ]" + " " + nameDesc + hint)
		} else {
			renderName := tuistyles.ItemStyle.Render
			if m.cursor == dp {
				renderName = tuistyles.SelectedItemStyle.Render
			} else if pendingRemoval {
				renderName = tuistyles.ErrorStyle.Render
			} else if checked {
				renderName = tuistyles.CheckedItemStyle.Render
			}

			versionStr := ""
			if m.versions[item.idx] != "" {
				versionStr = "  " + tuistyles.StatusStyle.Render(m.versions[item.idx])
			}

			rowContent = prefix + checkbox + " " + renderName(nameDesc) + versionStr
		}

		sb.WriteString(cursorStr + rowContent + "\n")
	}

	sb.WriteString("\n")

	btnIdx := len(m.displayOrder)
	btnStyle := tuistyles.ButtonStyle
	if m.cursor == btnIdx {
		btnStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(btnStyle.Render("  Continue  ") + "\n")

	return sb.String()
}

// viewPopup renders the removal confirmation popup.
func (m *ToolsModel) viewPopup() string {
	tool := m.tools[m.displayOrder[m.popupToolDP].idx]

	var inner strings.Builder
	inner.WriteString(tuistyles.WarningStyle.Render("Remove tool") + "\n\n")
	inner.WriteString(tuistyles.SelectedItemStyle.Render(tool.Name) + "  " +
		tuistyles.StatusStyle.Render(tool.Description) + "\n")

	if len(m.popupDeps) > 0 {
		inner.WriteString("\n" + tuistyles.StatusStyle.Render(
			"The following tools also require "+tool.Name+" and will be uninstalled:") + "\n")
		for _, dep := range m.popupDeps {
			inner.WriteString(tuistyles.WarningStyle.Render("  • "+dep) + "\n")
		}
	}
	if len(m.popupDeselectedDeps) > 0 {
		inner.WriteString("\n" + tuistyles.StatusStyle.Render(
			"The following selected tools also require "+tool.Name+" and will be deselected:") + "\n")
		for _, dep := range m.popupDeselectedDeps {
			inner.WriteString(tuistyles.StatusStyle.Render("  • "+dep) + "\n")
		}
	}

	inner.WriteString("\n" + tuistyles.StatusStyle.Render("Are you sure?") + "\n\n")

	yesStyle := tuistyles.ButtonStyle
	noStyle := tuistyles.ButtonStyle
	if m.popupCursor == 0 {
		yesStyle = tuistyles.ActiveButtonStyle
	} else {
		noStyle = tuistyles.ActiveButtonStyle
	}
	inner.WriteString(yesStyle.Render("  Yes, Remove  ") + "  " + noStyle.Render("  Cancel  ") + "\n")

	var sb strings.Builder
	sb.WriteString(tuistyles.ConfirmStyle.Render(inner.String()) + "\n")
	sb.WriteString("\n" + tuistyles.StatusStyle.Render("←/→: select  ENTER: confirm  ESC: cancel") + "\n")
	return sb.String()
}
