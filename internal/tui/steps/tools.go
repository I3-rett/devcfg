package steps

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/resolver"
	"github.com/I3-rett/devcfg/internal/system"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

type installResultMsg struct {
	name string
	err  error
}

type uninstallResultMsg struct {
	name string
	err  error
}

type toolDetectMsg struct {
	versions []string // one entry per tool; empty string means not installed
}

// logLineMsg carries one line of output from a running tool.
type logLineMsg struct {
	toolName string
	line     string
	ch       chan string // channel to re-read from (avoids stale-ref issues)
}

// logDoneMsg is sent when a tool's log channel is closed (command finished).
type logDoneMsg struct {
	toolName string
}

// pendingOp is one install or uninstall operation queued for execution.
type pendingOp struct {
	tool        registry.Tool
	isUninstall bool
}

// logChannelBufSize is the number of log lines buffered per operation.
// 1024 lines is large enough to absorb the typical burst output of package
// managers (brew, apt-get, etc.) without blocking the scanner goroutine while
// the Bubble Tea event loop catches up.
const logChannelBufSize = 1024

// logMaxLines is the maximum number of log lines retained per tool.
// Keeping a large history allows the user to scroll back through output.
const logMaxLines = 500

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
	results      []string
	errors       []string

	// Terminal dimensions (updated via tea.WindowSizeMsg).
	width  int
	height int

	// Sequential installation state.
	pendingOps      []pendingOp         // operations to execute in order
	opIdx           int                 // index of the currently running op
	opSuccess       []bool              // per-op success flag (set on completion)
	toolLogs        map[string][]string // accumulated log lines per tool name
	currentTool     string              // name of the tool currently being installed
	cancelFn        context.CancelFunc  // cancels the in-progress operation
	logScrollOffset int                 // lines scrolled up from the bottom (0 = follow tail)

	// Ctrl+C abort confirmation overlay (shown while running).
	abortMode   bool
	abortCursor int // 0 = "Yes, Abort"; 1 = "Continue"

	// confirmation popup state
	popupMode           bool     // removal confirmation popup is currently shown
	popupToolDP         int      // display position of the tool awaiting confirmation
	popupDeps           []string // names of INSTALLED checked tools that will also be uninstalled
	popupDeselectedDeps []string // names of checked-but-not-installed tools that will be deselected
	popupCursor         int      // 0 = "Yes, Remove"; 1 = "Cancel"
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

// Title returns the display name of this step.
func (m *ToolsModel) Title() string { return "Tools Installation" }

// IsDone reports whether the tool installation step has been completed.
func (m *ToolsModel) IsDone() bool { return m.done }

// CanQuit returns false while an installation is running or the abort
// confirmation is visible, so that Ctrl+C is handled by the model itself
// rather than immediately killing the program.
func (m *ToolsModel) CanQuit() bool { return !m.running && !m.abortMode }

// Init detects currently installed tool versions asynchronously.
func (m *ToolsModel) Init() tea.Cmd {
	tools := m.tools
	return func() tea.Msg {
		versions := make([]string, len(tools))
		for i, t := range tools {
			for _, bin := range t.BinaryNames() {
				if ver := system.DetectToolVersion(bin); ver != "" {
					versions[i] = ver
					break
				}
			}
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

// Update handles messages for the tools installation step.
func (m *ToolsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.popupMode {
		return m.updatePopupMsg(msg)
	}
	if m.running {
		return m.updateRunningMsg(msg)
	}
	return m.updateSelectionMsg(msg)
}

// updatePopupMsg handles messages while the removal confirmation popup is open.
func (m *ToolsModel) updatePopupMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
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
	return m, nil
}

// updateRunningMsg handles all messages received while an operation is in progress.
func (m *ToolsModel) updateRunningMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.abortMode {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			m.updateAbortKey(keyMsg.String())
		}
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.MouseMsg:
		m.updateScrollOffset(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.abortMode = true
			m.abortCursor = 1 // default to "Continue" (safer)
		}
	case logLineMsg:
		return m, m.handleLogLine(msg)
	case installResultMsg:
		return m, m.handleOpResult(msg.name, msg.err, true)
	case uninstallResultMsg:
		return m, m.handleOpResult(msg.name, msg.err, false)
	}
	return m, nil
}

// updateAbortKey processes a key press on the abort confirmation overlay.
func (m *ToolsModel) updateAbortKey(key string) {
	switch key {
	case "left", "h":
		m.abortCursor = 0
	case "right", "l":
		m.abortCursor = 1
	case " ", "enter":
		if m.abortCursor == 0 {
			if m.cancelFn != nil {
				m.cancelFn()
			}
			m.running = false
			m.done = true
			m.errors = append(m.errors, "⚠ Installation aborted by user")
		}
		m.abortMode = false
	case "esc":
		m.abortMode = false
	}
}

// updateScrollOffset adjusts the log scroll position in response to mouse wheel events.
func (m *ToolsModel) updateScrollOffset(msg tea.MouseMsg) {
	if msg.Action != tea.MouseActionPress {
		return
	}
	logs := m.toolLogs[m.currentTool]
	visibleLines := m.height - 10
	if visibleLines < 5 {
		visibleLines = 5
	}
	maxOffset := len(logs) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if m.logScrollOffset < maxOffset {
			m.logScrollOffset++
		}
	case tea.MouseButtonWheelDown:
		if m.logScrollOffset > 0 {
			m.logScrollOffset--
		}
	}
}

// handleLogLine appends one log line (capped at logMaxLines) and re-subscribes to the channel.
func (m *ToolsModel) handleLogLine(msg logLineMsg) tea.Cmd {
	logs := append(m.toolLogs[msg.toolName], msg.line)
	if len(logs) > logMaxLines {
		logs = logs[len(logs)-logMaxLines:]
	}
	m.toolLogs[msg.toolName] = logs
	return waitForLog(msg.toolName, msg.ch)
}

// handleOpResult records a completed install or uninstall result and starts the next operation.
func (m *ToolsModel) handleOpResult(name string, err error, isInstall bool) tea.Cmd {
	if m.cancelFn != nil {
		m.cancelFn() // release context resources on normal completion
	}
	if err != nil {
		m.errors = append(m.errors, fmt.Sprintf("✗ %s: %s", name, err.Error()))
		m.opSuccess[m.opIdx] = false
	} else {
		verb := "installed"
		if !isInstall {
			verb = "removed"
		}
		m.results = append(m.results, fmt.Sprintf("✓ %s %s", name, verb))
		m.opSuccess[m.opIdx] = true
	}
	m.opIdx++
	return m.startNextOp()
}

// updateSelectionMsg handles messages while the tool selection list is displayed.
func (m *ToolsModel) updateSelectionMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case toolDetectMsg:
		m.applyToolDetect(msg)
		return m, nil
	}
	if !m.loaded {
		return m, nil
	}
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		return m, m.updateSelectionKey(keyMsg.String())
	}
	return m, nil
}

// applyToolDetect stores detected versions, pre-checks installed tools, and
// propagates dependency constraints.
func (m *ToolsModel) applyToolDetect(msg toolDetectMsg) {
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
	m.cascadeUncheck()
	m.loaded = true
}

// updateSelectionKey handles keyboard navigation and selection in the tool list.
func (m *ToolsModel) updateSelectionKey(key string) tea.Cmd {
	continueIdx := len(m.displayOrder)
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < continueIdx {
			m.cursor++
		}
	case " ":
		if m.cursor < continueIdx {
			if m.isAvailable(m.cursor) {
				m.toggleOrConfirm(m.cursor)
			}
		}
	case "enter":
		if m.cursor < continueIdx {
			if m.isAvailable(m.cursor) {
				m.toggleOrConfirm(m.cursor)
			}
		} else {
			return m.startInstallation()
		}
	}
	return nil
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
	// Collect uninstalls in reverse display order (dependents before parents).
	var ops []pendingOp
	for i := len(m.displayOrder) - 1; i >= 0; i-- {
		item := m.displayOrder[i]
		if m.toUninstall[item.idx] {
			ops = append(ops, pendingOp{tool: m.tools[item.idx], isUninstall: true})
		}
	}
	// Collect installs in display order (parents before dependents).
	for dp, item := range m.displayOrder {
		if m.checked[item.idx] && m.versions[item.idx] == "" && m.isAvailable(dp) {
			ops = append(ops, pendingOp{tool: m.tools[item.idx], isUninstall: false})
		}
	}

	if len(ops) == 0 {
		m.done = true
		return nil
	}

	m.pendingOps = ops
	m.opIdx = 0
	m.opSuccess = make([]bool, len(ops))
	m.toolLogs = make(map[string][]string)
	m.running = true

	return m.startNextOp()
}

// startNextOp launches the next pending operation and returns a tea.Batch of
// the two commands that drive it: one to stream log lines and one to receive
// the final result.  When all operations are done it marks the model as done.
func (m *ToolsModel) startNextOp() tea.Cmd {
	if m.opIdx >= len(m.pendingOps) {
		m.running = false
		m.done = true
		return nil
	}

	op := m.pendingOps[m.opIdx]
	m.currentTool = op.tool.Name
	m.logScrollOffset = 0 // reset scroll to follow the new tool's output

	// Channel for streaming log lines (buffered to absorb bursts).
	logCh := make(chan string, logChannelBufSize)
	// Channel for the final error result.
	errCh := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelFn = cancel

	tool := op.tool
	isUninstall := op.isUninstall

	go func() {
		sysInfo := system.Detect()
		var args []string
		var err error
		if isUninstall {
			args, err = resolver.ResolveUninstall(tool, sysInfo)
		} else {
			args, err = resolver.Resolve(tool, sysInfo)
		}
		if err != nil {
			logCh <- "error: " + err.Error()
			close(logCh)
			errCh <- err
			return
		}
		res := executor.ExecuteWithContext(ctx, args, logCh)
		close(logCh)
		errCh <- res.Err
	}()

	toolName := op.tool.Name
	return tea.Batch(
		waitForLog(toolName, logCh),
		waitForDone(toolName, errCh, isUninstall),
	)
}

// waitForLog blocks on one read from ch and returns the line as a logLineMsg
// (or logDoneMsg when the channel is closed).  The channel is embedded in the
// message so that the model can re-issue the command without holding a
// reference that could be stale after the next operation starts.
func waitForLog(toolName string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logDoneMsg{toolName: toolName}
		}
		return logLineMsg{toolName: toolName, line: line, ch: ch}
	}
}

// waitForDone blocks until the error channel is written, then converts the
// result into the appropriate Bubble Tea result message.
func waitForDone(toolName string, errCh chan error, isUninstall bool) tea.Cmd {
	return func() tea.Msg {
		err := <-errCh
		if isUninstall {
			return uninstallResultMsg{name: toolName, err: err}
		}
		return installResultMsg{name: toolName, err: err}
	}
}

// View renders the tools installation step.
func (m *ToolsModel) View() string {
	if m.popupMode {
		return m.viewPopup()
	}
	if m.running {
		return m.viewRunning()
	}
	if m.done {
		return m.viewDone()
	}
	if !m.loaded {
		return tuistyles.StatusStyle.Render("Detecting installed tools...") + "\n"
	}
	return m.viewSelection()
}

// viewDone renders the summary screen shown after all operations complete.
func (m *ToolsModel) viewDone() string {
	var sb strings.Builder
	sb.WriteString(tuistyles.SuccessStyle.Render("Done!") + "\n\n")
	for _, r := range m.results {
		sb.WriteString(tuistyles.SuccessStyle.Render(r) + "\n")
	}
	for _, e := range m.errors {
		sb.WriteString(tuistyles.ErrorStyle.Render(e) + "\n")
	}
	return sb.String()
}

// viewSelection renders the interactive tool list with checkboxes and a Continue button.
func (m *ToolsModel) viewSelection() string {
	var sb strings.Builder
	sb.WriteString(tuistyles.StatusStyle.Render("Select tools to install/remove (SPACE/ENTER to toggle):") + "\n\n")
	for dp, item := range m.displayOrder {
		sb.WriteString(m.viewToolRow(dp, item))
	}
	sb.WriteString("\n")
	btnStyle := tuistyles.ButtonStyle
	if m.cursor == len(m.displayOrder) {
		btnStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(btnStyle.Render("  Continue  ") + "\n")
	return sb.String()
}

// viewToolRow renders one row of the tool selection list.
func (m *ToolsModel) viewToolRow(dp int, item toolItem) string {
	tool := m.tools[item.idx]
	available := m.isAvailable(dp)
	checked := m.checked[item.idx]
	pendingRemoval := m.toUninstall[item.idx]

	cursorStr := "  "
	if m.cursor == dp {
		cursorStr = tuistyles.SelectedItemStyle.Render("▶ ")
	}

	prefix := treePrefix(item)
	nameDesc := fmt.Sprintf("%-12s %s", tool.Name, tool.Description)

	if !available {
		hint := " [requires: " + strings.Join(tool.Requires, ", ") + "]"
		return cursorStr + tuistyles.DisabledItemStyle.Render(prefix+"[  ] "+nameDesc+hint) + "\n"
	}

	versionStr := ""
	if m.versions[item.idx] != "" {
		versionStr = "  " + tuistyles.StatusStyle.Render(m.versions[item.idx])
	}
	rowContent := prefix + checkboxStr(checked, pendingRemoval) + " " +
		m.rowRenderFn(dp, checked, pendingRemoval)(nameDesc) + versionStr
	return cursorStr + rowContent + "\n"
}

// checkboxStr returns the styled checkbox string for the given selection state.
func checkboxStr(checked, pendingRemoval bool) string {
	switch {
	case pendingRemoval:
		return tuistyles.ErrorStyle.Render("[✗]")
	case checked:
		return tuistyles.CheckedItemStyle.Render("[✓]")
	default:
		return "[ ]"
	}
}

// rowRenderFn returns the appropriate lipgloss render function for a tool row based on its state.
func (m *ToolsModel) rowRenderFn(dp int, checked, pendingRemoval bool) func(...string) string {
	switch {
	case m.cursor == dp:
		return tuistyles.SelectedItemStyle.Render
	case pendingRemoval:
		return tuistyles.ErrorStyle.Render
	case checked:
		return tuistyles.CheckedItemStyle.Render
	default:
		return tuistyles.ItemStyle.Render
	}
}

// viewRunning renders the split-screen layout shown while operations execute.
func (m *ToolsModel) viewRunning() string {
	if m.abortMode {
		return m.viewAbortConfirm()
	}
	totalWidth := m.width
	if totalWidth < 40 {
		totalWidth = 80 // sensible fallback before the first WindowSizeMsg
	}
	leftInner, rightInner := computePaneWidths(totalWidth)
	leftPane := tuistyles.OpPaneBorderStyle.Width(leftInner).Render(m.viewOpList())
	rightPane := tuistyles.LogPaneBorderStyle.Width(rightInner).Render(m.viewLogPane())
	scrollHint := ""
	if len(m.toolLogs[m.currentTool]) > 0 {
		scrollHint = "  scroll: mouse wheel"
	}
	result := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	return result + "\n" + tuistyles.StatusStyle.Render("Ctrl+C: abort"+scrollHint) + "\n"
}

// computePaneWidths calculates the inner widths of the two split-screen panes
// given the total terminal width.
func computePaneWidths(totalWidth int) (leftInner, rightInner int) {
	// Each RoundedBorder adds 2 chars (left + right), so inner = rendered - 2.
	innerTotal := totalWidth - 4 // 2 borders × 2 panes
	if innerTotal < 4 {
		innerTotal = 4
	}
	leftInner = innerTotal * 35 / 100
	if leftInner < 20 {
		leftInner = 20
	}
	rightInner = innerTotal - leftInner
	if rightInner < 10 {
		rightInner = 10
	}
	return
}

// viewOpList renders the left pane: a list of pending operations with status icons.
func (m *ToolsModel) viewOpList() string {
	var sb strings.Builder
	sb.WriteString(tuistyles.PaneTitleStyle.Render("Operations") + "\n")
	for i, op := range m.pendingOps {
		icon, nameStr := m.opStatusStrings(i, op)
		actionStr := tuistyles.StatusStyle.Render("(" + opActionLabel(op.isUninstall) + ")")
		fmt.Fprintf(&sb, " %s %s %s\n", icon, nameStr, actionStr)
	}
	return sb.String()
}

// opActionLabel returns the human-readable action label for an operation.
func opActionLabel(isUninstall bool) string {
	if isUninstall {
		return "remove"
	}
	return "install"
}

// opStatusStrings returns the styled icon and tool name for the operation at index i.
func (m *ToolsModel) opStatusStrings(i int, op pendingOp) (icon, nameStr string) {
	switch {
	case i < m.opIdx:
		if m.opSuccess[i] {
			return tuistyles.SuccessStyle.Render("✓"), tuistyles.SuccessStyle.Render(op.tool.Name)
		}
		return tuistyles.ErrorStyle.Render("✗"), tuistyles.ErrorStyle.Render(op.tool.Name)
	case i == m.opIdx:
		return tuistyles.WarningStyle.Render("▶"), tuistyles.SelectedItemStyle.Render(op.tool.Name)
	default:
		return tuistyles.StatusStyle.Render("○"), tuistyles.ItemStyle.Render(op.tool.Name)
	}
}

// viewLogPane renders the right pane: live log output for the current tool.
func (m *ToolsModel) viewLogPane() string {
	var sb strings.Builder
	logTitle := "Logs"
	if m.currentTool != "" {
		logTitle = "Logs: " + m.currentTool
	}
	sb.WriteString(tuistyles.PaneTitleStyle.Render(logTitle) + "\n")
	logs := m.toolLogs[m.currentTool]
	if len(logs) == 0 {
		sb.WriteString(tuistyles.StatusStyle.Render("Waiting for output...") + "\n")
	} else {
		m.appendScrolledLogs(&sb, logs)
	}
	return sb.String()
}

// appendScrolledLogs writes a windowed, scroll-adjusted slice of log lines into sb,
// followed by a scroll indicator when the user has scrolled up from the tail.
func (m *ToolsModel) appendScrolledLogs(sb *strings.Builder, logs []string) {
	visibleLines := m.height - 10
	if visibleLines < 5 {
		visibleLines = 5
	}
	maxOffset := len(logs) - visibleLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset := m.logScrollOffset
	if offset > maxOffset {
		offset = maxOffset
	}
	start := len(logs) - visibleLines - offset
	if start < 0 {
		start = 0
	}
	end := start + visibleLines
	if end > len(logs) {
		end = len(logs)
	}
	for _, line := range logs[start:end] {
		sb.WriteString(tuistyles.StatusStyle.Render(line) + "\n")
	}
	if offset > 0 {
		sb.WriteString(tuistyles.StatusStyle.Render(
			fmt.Sprintf("↑ %d more line(s) below (scroll ↓ to follow)", offset),
		) + "\n")
	}
}

// viewAbortConfirm renders the Ctrl+C abort confirmation overlay.
func (m *ToolsModel) viewAbortConfirm() string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(tuistyles.WarningStyle.Render("⚠  Installation in progress. Abort?") + "\n\n")

	yesStyle := tuistyles.ButtonStyle
	noStyle := tuistyles.ButtonStyle
	if m.abortCursor == 0 {
		yesStyle = tuistyles.ActiveButtonStyle
	} else {
		noStyle = tuistyles.ActiveButtonStyle
	}
	sb.WriteString(yesStyle.Render("  Yes, Abort  ") + "  " + noStyle.Render("  Continue  ") + "\n\n")
	sb.WriteString(tuistyles.StatusStyle.Render("←/→ or h/l: select  ENTER/SPACE: confirm  ESC: dismiss") + "\n")
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
