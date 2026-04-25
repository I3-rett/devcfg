package steps

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/I3-rett/devcfg/internal/executor"
	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/resolver"
	"github.com/I3-rett/devcfg/internal/system"
	"github.com/I3-rett/devcfg/internal/tui/tuistyles"
)

// ── message types ────────────────────────────────────────────────────────────

type installResultMsg struct {
	name string
	err  error
}

type uninstallResultMsg struct {
	name string
	err  error
}

type toolDetectMsg struct {
	versions []string
}

type logLineMsg struct {
	toolName string
	line     string
	ch       chan string
}

type logDoneMsg struct {
	toolName string
}

// ptyStartedMsg is sent once the PTY is open and ready for the active op.
type ptyStartedMsg struct {
	toolName    string
	isUninstall bool
	ptm         *os.File
	logCh       chan string
	errCh       <-chan error
}

// ptyStartFailedMsg is sent when the command cannot be started.
type ptyStartFailedMsg struct {
	toolName string
	err      error
}

// ── constants ────────────────────────────────────────────────────────────────

const logChannelBufSize = 1024
const logMaxLines = 500

// UI layout constants for height calculations.
const (
	// appUIReservedRows is the number of rows AppModel reserves for tab bar and footer.
	// Tab bar (1 line) + footer hints (2 lines) = 3 rows total.
	appUIReservedRows = 3

	// paneBorderRows is the number of rows consumed by pane borders (top + bottom).
	paneBorderRows = 2

	// paneTitleRows is the number of rows consumed by the pane title.
	// Title text (1 line) + PaneTitleStyle.MarginBottom (1 line) + explicit "\n" (1 line) = 3 rows.
	paneTitleRows = 3

	// splitViewHintRows is the number of rows for hints below the split view.
	// Hint text (1 line) + final newline (1 line) = 2 rows.
	splitViewHintRows = 2
)

// ── helper types ─────────────────────────────────────────────────────────────

// toolItem is one row in the tree-shaped display list.
type toolItem struct {
	idx             int
	depth           int
	isLast          bool
	parentContinues []bool
}

// pendingOp is one install or uninstall operation.
type pendingOp struct {
	tool        registry.Tool
	isUninstall bool
}

// opResult records the outcome of a completed operation.
type opResult struct {
	name        string
	isUninstall bool
	success     bool
	err         error
}

// ── tree helpers ─────────────────────────────────────────────────────────────

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

// ── model ────────────────────────────────────────────────────────────────────

// ToolsModel is the Bubble Tea model for the tool-installation tab.
type ToolsModel struct {
	tools        []registry.Tool
	nameToIdx    map[string]int
	displayOrder []toolItem

	loaded   bool
	cursor   int
	versions []string // "" = not installed

	// Active operation (nil PTY = idle).
	activeName        string
	activeIsUninstall bool
	activeCancel      context.CancelFunc
	activePty         *os.File

	// Queued operations (run sequentially after active op).
	opQueue []pendingOp

	// PTY interaction.
	ptyFocused bool

	// Log output per tool name.
	toolLogs        map[string][]string
	logScrollOffset int

	// Completed op history.
	completedOps []opResult

	// Removal confirmation popup.
	popupMode   bool
	popupToolDP int
	popupDeps   []string // installed tools that will also be uninstalled
	popupCursor int      // 0 = "Yes, Remove", 1 = "Cancel"

	sysInfo system.Info
	width   int
	height  int
}

// NewToolsModel creates and returns a new ToolsModel initialised with the
// tool registry and system information used to resolve install commands.
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
		versions:     make([]string, len(tools)),
		toolLogs:     make(map[string][]string),
		sysInfo:      sysInfo,
	}
}

// Title returns the display name of the Tools tab.
func (m *ToolsModel) Title() string { return "Tools" }

// IsDone reports whether the Tools tab has completed all its work.
func (m *ToolsModel) IsDone() bool { return false }

// CanQuit returns false while the PTY is focused or work is in progress so
// Ctrl+C is forwarded to the running process and in-flight operations can be
// cancelled/cleaned up predictably by the model.
func (m *ToolsModel) CanQuit() bool { return !m.ptyFocused && !m.isBusy() }

// CanSwitchTabs returns false when a popup is open, the PTY is focused,
// or install/uninstall work is still active or queued.
func (m *ToolsModel) CanSwitchTabs() bool { return !m.popupMode && !m.ptyFocused && !m.isBusy() }

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

// ── availability & dependency helpers ────────────────────────────────────────

// isInstalled reports whether the tool at toolIdx has a detected version.
func (m *ToolsModel) isInstalled(toolIdx int) bool {
	return m.versions[toolIdx] != ""
}

// isBusy reports whether any operation is currently active or queued.
func (m *ToolsModel) isBusy() bool {
	return m.activeName != "" || len(m.opQueue) > 0
}

// isAvailable reports whether all required tools for the item at displayPos
// are currently installed.
func (m *ToolsModel) isAvailable(displayPos int) bool {
	tool := m.tools[m.displayOrder[displayPos].idx]
	for _, req := range tool.Requires {
		reqIdx, ok := m.nameToIdx[req]
		if !ok {
			continue
		}
		if !m.isInstalled(reqIdx) {
			return false
		}
	}
	return true
}

// alreadyQueuedByIdx reports whether the tool at toolIdx is the active op or
// is already present in the queue (as an install).
func (m *ToolsModel) alreadyQueuedByIdx(toolIdx int) bool {
	name := m.tools[toolIdx].Name
	if m.activeName == name {
		return true
	}
	for _, op := range m.opQueue {
		if op.tool.Name == name && !op.isUninstall {
			return true
		}
	}
	return false
}

// collectInstallOrder returns the ops needed to install the tool at toolIdx,
// including any missing dependencies in depth-first order.  Already-installed
// and already-queued tools are skipped.
func (m *ToolsModel) collectInstallOrder(toolIdx int, visited map[int]bool) []pendingOp {
	if visited[toolIdx] {
		return nil
	}
	visited[toolIdx] = true

	var ops []pendingOp
	for _, req := range m.tools[toolIdx].Requires {
		reqIdx, ok := m.nameToIdx[req]
		if !ok {
			continue
		}
		if m.isInstalled(reqIdx) || m.alreadyQueuedByIdx(reqIdx) {
			continue
		}
		ops = append(ops, m.collectInstallOrder(reqIdx, visited)...)
	}
	if !m.isInstalled(toolIdx) && !m.alreadyQueuedByIdx(toolIdx) {
		ops = append(ops, pendingOp{tool: m.tools[toolIdx], isUninstall: false})
	}
	return ops
}

// installedDependentsOf returns the names of installed tools that transitively
// depend on the tool at toolIdx, ordered so the most-dependent tools come first
// (safe uninstall order: dependents before the tool they depend on).
func (m *ToolsModel) installedDependentsOf(toolIdx int) []string {
	target := m.tools[toolIdx].Name
	visited := make(map[string]bool)
	var order []string

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		// Find installed tools that directly depend on 'name' and recurse.
		for i, t := range m.tools {
			if !m.isInstalled(i) {
				continue
			}
			for _, req := range t.Requires {
				if req == name {
					visit(t.Name)
					break
				}
			}
		}
		if name != target {
			order = append(order, name)
		}
	}
	visit(target)
	return order
}

// ── queue & op management ────────────────────────────────────────────────────

// enqueueInstall queues the install of the tool at displayPos and all missing
// deps, then starts the queue if idle.
func (m *ToolsModel) enqueueInstall(dp int) tea.Cmd {
	toolIdx := m.displayOrder[dp].idx
	ops := m.collectInstallOrder(toolIdx, make(map[int]bool))
	m.opQueue = append(m.opQueue, ops...)
	if m.activeName == "" {
		return m.startFromQueue()
	}
	return nil
}

// enqueueUninstall queues the uninstall of the tool at displayPos (and
// installed dependents) then starts the queue if idle.
func (m *ToolsModel) enqueueUninstall(dp int) tea.Cmd {
	toolIdx := m.displayOrder[dp].idx
	// Uninstall dependents first (reverse dep order).
	for _, depName := range m.installedDependentsOf(toolIdx) {
		if depIdx, ok := m.nameToIdx[depName]; ok {
			m.opQueue = append(m.opQueue, pendingOp{tool: m.tools[depIdx], isUninstall: true})
		}
	}
	m.opQueue = append(m.opQueue, pendingOp{tool: m.tools[toolIdx], isUninstall: true})
	if m.activeName == "" {
		return m.startFromQueue()
	}
	return nil
}

// startFromQueue dequeues the next op and starts it.  Returns nil when the
// queue is empty (all done).
func (m *ToolsModel) startFromQueue() tea.Cmd {
	if len(m.opQueue) == 0 {
		return nil
	}
	op := m.opQueue[0]
	m.opQueue = m.opQueue[1:]
	return m.startOp(op)
}

// startOp launches the given op inside a PTY and returns the tea.Cmd that
// kicks off the async start.
func (m *ToolsModel) startOp(op pendingOp) tea.Cmd {
	m.activeName = op.tool.Name
	m.activeIsUninstall = op.isUninstall
	m.logScrollOffset = 0

	tool := op.tool
	isUninstall := op.isUninstall
	sysInfo := m.sysInfo

	ctx, cancel := context.WithCancel(context.Background())
	m.activeCancel = cancel

	return func() tea.Msg {
		var args []string
		var err error
		if isUninstall {
			args, err = resolver.ResolveUninstall(tool, sysInfo)
		} else {
			args, err = resolver.Resolve(tool, sysInfo)
		}
		if err != nil {
			return ptyStartFailedMsg{toolName: tool.Name, err: err}
		}

		logCh := make(chan string, logChannelBufSize)
		ptm, errCh, startErr := executor.ExecuteWithPTY(ctx, args, logCh)
		if startErr != nil {
			// Fallback: pipe-based execution (no interactive input).
			pipeLogCh := make(chan string, logChannelBufSize)
			pipeErrCh := make(chan error, 1)
			go func() {
				res := executor.ExecuteWithContext(ctx, args, pipeLogCh)
				close(pipeLogCh)
				pipeErrCh <- res.Err
			}()
			return ptyStartedMsg{
				toolName: tool.Name, isUninstall: isUninstall,
				ptm: nil, logCh: pipeLogCh, errCh: pipeErrCh,
			}
		}
		return ptyStartedMsg{
			toolName: tool.Name, isUninstall: isUninstall,
			ptm: ptm, logCh: logCh, errCh: errCh,
		}
	}
}

// handleOpDone records the result, updates the version map, and starts the
// next queued op.
func (m *ToolsModel) handleOpDone(name string, err error, isUninstall bool) tea.Cmd {
	if m.activePty != nil {
		_ = m.activePty.Close()
		m.activePty = nil
	}
	if m.activeCancel != nil {
		m.activeCancel()
		m.activeCancel = nil
	}
	m.ptyFocused = false

	m.completedOps = append(m.completedOps, opResult{
		name:        name,
		isUninstall: isUninstall,
		success:     err == nil,
		err:         err,
	})

	if err == nil {
		// Re-detect version for the affected tool.
		if idx, ok := m.nameToIdx[name]; ok {
			if isUninstall {
				m.versions[idx] = ""
			} else {
				// Quick re-probe; fall back to "(installed)" if version check fails.
				tool := m.tools[idx]
				for _, bin := range tool.BinaryNames() {
					if ver := system.DetectToolVersion(bin); ver != "" {
						m.versions[idx] = ver
						break
					}
				}
				if m.versions[idx] == "" {
					m.versions[idx] = name + " (installed)"
				}
			}
		}
	}

	m.activeName = ""
	m.activeIsUninstall = false
	return m.startFromQueue()
}

// ── Update ───────────────────────────────────────────────────────────────────

// Update dispatches incoming Bubble Tea messages to the appropriate handler.
func (m *ToolsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.popupMode {
		return m.updatePopup(msg)
	}
	return m.updateMain(msg)
}

func (m *ToolsModel) updatePopup(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "left", "h":
		m.popupCursor = 0
	case "right", "l":
		m.popupCursor = 1
	case " ", "enter":
		if m.popupCursor == 0 {
			m.popupMode = false
			return m, m.enqueueUninstall(m.popupToolDP)
		}
		m.popupMode = false
	case "esc":
		m.popupMode = false
	}
	return m, nil
}

func (m *ToolsModel) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.MouseMsg:
		m.handleMouseInput(msg)
	case tea.KeyMsg:
		return m, m.handleKeyInput(msg)
	case toolDetectMsg:
		m.handleToolDetect(msg)
	case ptyStartedMsg:
		m.activePty = msg.ptm
		m.ptyFocused = true // Auto-focus logs pane when installation starts
		return m, tea.Batch(
			waitForLog(msg.toolName, msg.logCh),
			waitForDone(msg.toolName, msg.errCh, msg.isUninstall),
		)
	case ptyStartFailedMsg:
		return m, m.handleOpDone(msg.toolName, msg.err, m.activeIsUninstall)
	case logLineMsg:
		return m, m.handleLogLine(msg)
	case logDoneMsg:
		// Channel closed; no re-subscription needed.
	case installResultMsg:
		return m, m.handleOpDone(msg.name, msg.err, false)
	case uninstallResultMsg:
		return m, m.handleOpDone(msg.name, msg.err, true)
	}
	return m, nil
}

// handleMouseInput toggles PTY focus based on which pane was clicked and
// updates the scroll offset for wheel events.
func (m *ToolsModel) handleMouseInput(msg tea.MouseMsg) {
	if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && m.activePty != nil {
		if msg.X >= m.leftPaneBoundary() {
			m.ptyFocused = true
		} else {
			m.ptyFocused = false
		}
	}
	m.updateScroll(msg)
}

// handleKeyInput forwards keystrokes to the PTY when focused, or delegates to
// the standard key handler otherwise.
func (m *ToolsModel) handleKeyInput(msg tea.KeyMsg) tea.Cmd {
	if m.ptyFocused && m.activePty != nil {
		if b := keyToPTYBytes(msg); b != nil {
			_, _ = m.activePty.Write(b)
		}
		// ESC unfocuses PTY so the user can navigate tabs again.
		if msg.Type == tea.KeyEsc {
			m.ptyFocused = false
		}
		return nil
	}
	return m.updateKey(msg.String())
}

// handleToolDetect stores the detected versions from a toolDetectMsg.
func (m *ToolsModel) handleToolDetect(msg toolDetectMsg) {
	n := len(m.tools)
	if len(msg.versions) < n {
		n = len(msg.versions)
	}
	for i := 0; i < n; i++ {
		m.versions[i] = msg.versions[i]
	}
	m.loaded = true
}

func (m *ToolsModel) updateKey(key string) tea.Cmd {
	if !m.loaded {
		return nil
	}
	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.displayOrder)-1 {
			m.cursor++
		}
	case " ", "enter":
		if !m.loaded || m.isBusy() {
			return nil
		}
		dp := m.cursor
		toolIdx := m.displayOrder[dp].idx

		if m.isInstalled(toolIdx) {
			// Confirm before uninstalling.
			m.popupDeps = m.installedDependentsOf(toolIdx)
			m.popupToolDP = dp
			m.popupCursor = 0
			m.popupMode = true
			return nil
		}
		// Install: auto-queue missing deps then the tool.
		return m.enqueueInstall(dp)
	}
	return nil
}

func (m *ToolsModel) updateScroll(msg tea.MouseMsg) {
	if msg.Action != tea.MouseActionPress {
		return
	}
	logs := m.toolLogs[m.currentLogTool()]
	visibleLines := m.computeVisibleLogLines()
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

func (m *ToolsModel) handleLogLine(msg logLineMsg) tea.Cmd {
	logs := append(m.toolLogs[msg.toolName], msg.line)
	if len(logs) > logMaxLines {
		logs = logs[len(logs)-logMaxLines:]
	}
	m.toolLogs[msg.toolName] = logs
	return waitForLog(msg.toolName, msg.ch)
}

// ── channel helpers ───────────────────────────────────────────────────────────

func waitForLog(toolName string, ch chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logDoneMsg{toolName: toolName}
		}
		return logLineMsg{toolName: toolName, line: line, ch: ch}
	}
}

func waitForDone(toolName string, errCh <-chan error, isUninstall bool) tea.Cmd {
	return func() tea.Msg {
		err := <-errCh
		if isUninstall {
			return uninstallResultMsg{name: toolName, err: err}
		}
		return installResultMsg{name: toolName, err: err}
	}
}

// ── PTY key forwarding ────────────────────────────────────────────────────────

// keyToEscSeq maps cursor/navigation keys to their VT100 escape sequences.
func keyToEscSeq(t tea.KeyType) []byte {
	switch t {
	case tea.KeyUp:
		return []byte("\x1b[A")
	case tea.KeyDown:
		return []byte("\x1b[B")
	case tea.KeyRight:
		return []byte("\x1b[C")
	case tea.KeyLeft:
		return []byte("\x1b[D")
	case tea.KeyHome:
		return []byte("\x1b[H")
	case tea.KeyEnd:
		return []byte("\x1b[F")
	case tea.KeyPgUp:
		return []byte("\x1b[5~")
	case tea.KeyPgDown:
		return []byte("\x1b[6~")
	case tea.KeyDelete:
		return []byte("\x1b[3~")
	}
	return nil
}

func keyToPTYBytes(msg tea.KeyMsg) []byte {
	switch msg.Type {
	case tea.KeyRunes:
		return []byte(string(msg.Runes))
	case tea.KeyEnter:
		return []byte("\r")
	case tea.KeyBackspace:
		return []byte("\x7f")
	case tea.KeyTab:
		return []byte("\t")
	case tea.KeySpace:
		return []byte(" ")
	case tea.KeyCtrlC:
		return []byte("\x03")
	case tea.KeyCtrlD:
		return []byte("\x04")
	case tea.KeyCtrlZ:
		return []byte("\x1a")
	case tea.KeyEsc:
		return []byte("\x1b")
	}
	return keyToEscSeq(msg.Type)
}

// ── View ──────────────────────────────────────────────────────────────────────

// View renders the Tools tab content.
func (m *ToolsModel) View() string {
	if m.popupMode {
		return m.viewPopup()
	}
	if !m.loaded {
		return tuistyles.StatusStyle.Render("Detecting installed tools...") + "\n"
	}

	logTool := m.currentLogTool()
	if logTool != "" {
		return m.viewSplit(logTool)
	}
	return m.viewList()
}

// currentLogTool returns the tool name whose logs should be shown, or "".
func (m *ToolsModel) currentLogTool() string {
	if m.activeName != "" {
		return m.activeName
	}
	if len(m.completedOps) > 0 {
		return m.completedOps[len(m.completedOps)-1].name
	}
	return ""
}

// leftPaneBoundary returns the screen X offset where the right (log) pane starts.
func (m *ToolsModel) leftPaneBoundary() int {
	totalWidth := m.width
	if totalWidth < 40 {
		totalWidth = 80
	}
	leftInner, _ := computePaneWidths(totalWidth)
	return leftInner + 2 // +2 for the rounded border
}

// viewList renders the full-width tool list (no active install).
func (m *ToolsModel) viewList() string {
	var sb strings.Builder
	busy := m.isBusy()
	hint := "ENTER: install/uninstall  ↑/↓: navigate"
	if busy {
		hint = "↑/↓: navigate  (installation in progress)"
	}
	sb.WriteString(tuistyles.StatusStyle.Render(hint) + "\n\n")

	for dp, item := range m.displayOrder {
		sb.WriteString(m.viewToolRow(dp, item, false))
	}

	if len(m.completedOps) > 0 {
		sb.WriteString("\n")
		for _, r := range m.completedOps {
			if r.success {
				verb := "installed"
				if r.isUninstall {
					verb = "removed"
				}
				sb.WriteString(tuistyles.SuccessStyle.Render(fmt.Sprintf("✓ %s %s", r.name, verb)) + "\n")
			} else {
				sb.WriteString(tuistyles.ErrorStyle.Render(fmt.Sprintf("✗ %s: %s", r.name, r.err)) + "\n")
			}
		}
	}
	return sb.String()
}

// viewSplit renders the split-screen layout: tool list left, log pane right.
func (m *ToolsModel) viewSplit(logTool string) string {
	totalWidth := m.width
	if totalWidth < 40 {
		totalWidth = 80
	}
	leftInner, rightInner := computePaneWidths(totalWidth)

	// Calculate pane height to keep both panes aligned.
	paneHeight := m.computePaneHeight()

	leftPane := tuistyles.OpPaneBorderStyle.Width(leftInner).Height(paneHeight).Render(m.viewToolListPane())
	rightPane := tuistyles.LogPaneBorderStyle.Width(rightInner).Height(paneHeight).Render(m.viewLogPane(logTool))

	result := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	var hint string
	if m.ptyFocused {
		hint = "ESC: unfocus terminal   (typing goes to the process)"
	} else if m.activePty != nil {
		hint = "Click log pane to interact with the process"
	}
	if hint != "" {
		result += "\n" + tuistyles.StatusStyle.Render(hint)
	}
	return result + "\n"
}

// viewToolListPane renders the tool list content for the left pane.
func (m *ToolsModel) viewToolListPane() string {
	var sb strings.Builder
	sb.WriteString(tuistyles.PaneTitleStyle.Render("Tools") + "\n")
	busy := m.isBusy()
	for dp, item := range m.displayOrder {
		sb.WriteString(m.viewToolRow(dp, item, busy))
	}
	return sb.String()
}

// viewToolRow renders one tool row.  When dimmed is true (install in progress)
// tools are greyed and not interactive.
func (m *ToolsModel) viewToolRow(dp int, item toolItem, dimmed bool) string {
	toolIdx := item.idx
	tool := m.tools[toolIdx]
	installed := m.isInstalled(toolIdx)
	isActive := m.activeName == tool.Name
	isQueued := func() bool {
		for _, op := range m.opQueue {
			if op.tool.Name == tool.Name {
				return true
			}
		}
		return false
	}()

	prefix := treePrefix(item)
	nameDesc := fmt.Sprintf("%-12s %s", tool.Name, tool.Description)

	// Status icon
	var icon string
	switch {
	case isActive:
		icon = tuistyles.WarningStyle.Render("[▶]")
	case isQueued:
		icon = tuistyles.StatusStyle.Render("[…]")
	case installed:
		icon = tuistyles.CheckedItemStyle.Render("[✓]")
	default:
		icon = "[ ]"
	}

	// Row styling
	cursorStr := "  "
	var rowStyle func(...string) string

	if dimmed && !isActive {
		rowStyle = tuistyles.DisabledItemStyle.Render
	} else if m.cursor == dp && !dimmed {
		cursorStr = tuistyles.SelectedItemStyle.Render("▶ ")
		rowStyle = tuistyles.SelectedItemStyle.Render
	} else if installed {
		rowStyle = tuistyles.CheckedItemStyle.Render
	} else {
		rowStyle = tuistyles.ItemStyle.Render
	}

	versionStr := ""
	if m.versions[toolIdx] != "" {
		versionStr = "  " + tuistyles.StatusStyle.Render(m.versions[toolIdx])
	}

	return cursorStr + icon + " " + rowStyle(prefix+nameDesc) + versionStr + "\n"
}

// viewLogPane renders the right pane with PTY output.
func (m *ToolsModel) viewLogPane(toolName string) string {
	var sb strings.Builder

	title := "Logs"
	if toolName != "" {
		title = "Logs: " + toolName
	}
	if m.ptyFocused {
		title += "  [FOCUSED]"
	}
	sb.WriteString(tuistyles.PaneTitleStyle.Render(title) + "\n")

	logs := m.toolLogs[toolName]
	if len(logs) == 0 {
		sb.WriteString(tuistyles.StatusStyle.Render("Waiting for output...") + "\n")
	} else {
		m.appendScrolledLogs(&sb, logs)
	}
	return sb.String()
}

func (m *ToolsModel) appendScrolledLogs(sb *strings.Builder, logs []string) {
	visibleLines := m.computeVisibleLogLines()

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

// viewPopup renders the removal confirmation popup.
func (m *ToolsModel) viewPopup() string {
	tool := m.tools[m.displayOrder[m.popupToolDP].idx]

	var inner strings.Builder
	inner.WriteString(tuistyles.WarningStyle.Render("Remove tool") + "\n\n")
	inner.WriteString(tuistyles.SelectedItemStyle.Render(tool.Name) + "  " +
		tuistyles.StatusStyle.Render(tool.Description) + "\n")

	if len(m.popupDeps) > 0 {
		inner.WriteString("\n" + tuistyles.StatusStyle.Render(
			"The following installed tools also require "+tool.Name+" and will be removed:") + "\n")
		for _, dep := range m.popupDeps {
			inner.WriteString(tuistyles.WarningStyle.Render("  • "+dep) + "\n")
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

// ── pane layout ───────────────────────────────────────────────────────────────

func computePaneWidths(totalWidth int) (leftInner, rightInner int) {
	innerTotal := totalWidth - 4
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

// computePaneHeight calculates the height for both panes in the split view.
// This ensures both panes remain aligned regardless of content.
func (m *ToolsModel) computePaneHeight() int {
	// The ToolsModel receives the full terminal height but must account for:
	// 1. AppModel UI (tab bar + footer hints)
	// 2. Split view hints below the panes
	availableHeight := m.height - appUIReservedRows - splitViewHintRows
	if availableHeight < 5 {
		// Minimum viable height to avoid negative or zero values
		availableHeight = 5
	}
	return availableHeight
}

// computeVisibleLogLines calculates how many log lines can fit in the log pane.
// Content height = pane height - borders - title header.
func (m *ToolsModel) computeVisibleLogLines() int {
	paneHeight := m.computePaneHeight()
	// Subtract space for borders and title (which includes MarginBottom and explicit "\n")
	contentHeight := paneHeight - paneBorderRows - paneTitleRows
	if contentHeight < 1 {
		// Ensure at least 1 line is visible
		contentHeight = 1
	}
	return contentHeight
}
