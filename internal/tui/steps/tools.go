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

type toolDetectMsg struct {
	versions []string // one entry per tool; empty string means not installed
}

// toolItem is one row in the tree-shaped display list.
type toolItem struct {
	idx             int    // index in ToolsModel.tools
	depth           int    // 0 = root, 1 = first-level child, …
	isLast          bool   // last sibling at this depth level
	parentContinues []bool // for each ancestor at depth ≥ 1, whether it had more siblings
}

// treePrefix builds the visual tree connector string for a toolItem.
// depth=0 → empty; depth=1 → "├── " or "└── "; depth=2 → "│   ├── " etc.
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
// not in the registry) are roots.
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

	var order []toolItem
	var addItem func(idx, depth int, isLast bool, parentContinues []bool)
	addItem = func(idx, depth int, isLast bool, parentContinues []bool) {
		order = append(order, toolItem{
			idx:             idx,
			depth:           depth,
			isLast:          isLast,
			parentContinues: parentContinues,
		})
		children := childrenOf[tools[idx].Name]
		for j, childIdx := range children {
			childIsLast := j == len(children)-1
			// parentContinues for depth-1 child: ancestors at depth≥1 only.
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

type ToolsModel struct {
	tools        []registry.Tool
	nameToIdx    map[string]int // tool name → index in tools
	displayOrder []toolItem     // tree-ordered display items
	checked      []bool         // indexed by tool index
	versions     []string       // indexed by tool index; empty = not installed
	loaded       bool
	cursor       int // position in displayOrder (0 … len(displayOrder) = Continue)
	sysInfo      system.Info
	done         bool
	running      bool
	installCount int // number of tools being installed
	results      []string
	errors       []string
	msgLines     []string
}

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
// displayPos are either already installed (version detected) or checked.
func (m *ToolsModel) isAvailable(displayPos int) bool {
tool := m.tools[m.displayOrder[displayPos].idx]
for _, req := range tool.Requires {
reqIdx, ok := m.nameToIdx[req]
if !ok {
continue
}
if m.versions[reqIdx] == "" && !m.checked[reqIdx] {
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

// setChecked toggles the checked state and propagates unchecks to dependents.
func (m *ToolsModel) setChecked(displayPos int, val bool) {
m.checked[m.displayOrder[displayPos].idx] = val
if !val {
m.cascadeUncheck()
}
}

func (m *ToolsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
if m.running {
switch msg := msg.(type) {
case installResultMsg:
if msg.err != nil {
m.errors = append(m.errors, fmt.Sprintf("✗ %s: %s", msg.name, msg.err.Error()))
} else {
m.results = append(m.results, fmt.Sprintf("✓ %s installed", msg.name))
}
m.msgLines = append(m.msgLines, msg.output)
if len(m.results)+len(m.errors) >= m.installCount {
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
m.setChecked(m.cursor, !m.checked[m.displayOrder[m.cursor].idx])
}
case "enter":
if m.cursor < continueIdx {
if m.isAvailable(m.cursor) {
m.setChecked(m.cursor, !m.checked[m.displayOrder[m.cursor].idx])
}
} else {
return m, m.startInstallation()
}
}
}
return m, nil
}

func (m *ToolsModel) startInstallation() tea.Cmd {
// Collect tools in display order so dependencies are installed before dependents.
var selected []registry.Tool
for dp, item := range m.displayOrder {
if m.checked[item.idx] && m.isAvailable(dp) {
selected = append(selected, m.tools[item.idx])
}
}
if len(selected) == 0 {
m.done = true
return nil
}
m.running = true
m.installCount = len(selected)
cmds := make([]tea.Cmd, len(selected))
for i, tool := range selected {
t := tool
cmds[i] = func() tea.Msg {
args, err := resolver.Resolve(t, m.sysInfo)
if err != nil {
return installResultMsg{name: t.Name, err: err}
}
res := executor.Execute(args)
return installResultMsg{name: t.Name, output: res.Output, err: res.Err}
}
}
return tea.Sequence(cmds...)
}

func (m *ToolsModel) View() string {
var sb strings.Builder

if m.running {
sb.WriteString(tuistyles.StatusStyle.Render("Installing selected tools...") + "\n\n")
for _, r := range m.results {
sb.WriteString(tuistyles.SuccessStyle.Render(r) + "\n")
}
for _, e := range m.errors {
sb.WriteString(tuistyles.ErrorStyle.Render(e) + "\n")
}
return sb.String()
}

if m.done {
sb.WriteString(tuistyles.SuccessStyle.Render("Installation complete!") + "\n\n")
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

sb.WriteString(tuistyles.StatusStyle.Render("Select tools to install (SPACE/ENTER to toggle):") + "\n\n")

for dp, item := range m.displayOrder {
tool := m.tools[item.idx]
available := m.isAvailable(dp)
checked := m.checked[item.idx]

cursorStr := "  "
if m.cursor == dp {
cursorStr = tuistyles.SelectedItemStyle.Render("▶ ")
}

checkbox := "[ ]"
if checked {
checkbox = "[✓]"
}

prefix := treePrefix(item)
nameDesc := fmt.Sprintf("%-12s %s", tool.Name, tool.Description)

var rowContent string
if !available {
hint := " [requires: " + strings.Join(tool.Requires, ", ") + "]"
rowContent = tuistyles.DisabledItemStyle.Render(prefix+checkbox+" "+nameDesc+hint)
} else {
renderName := tuistyles.ItemStyle.Render
if m.cursor == dp {
renderName = tuistyles.SelectedItemStyle.Render
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
