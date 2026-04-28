package steps

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/system"
)

// newLoadedModel returns a ToolsModel that has received a toolDetectMsg
// with the given versions slice (one entry per tool from registry.List()).
func newLoadedModel(versions []string) *ToolsModel {
	m := NewToolsModel(system.Info{OS: "linux", PackageManager: "apt"})
	updated, _ := m.Update(toolDetectMsg{versions: versions})
	return updated.(*ToolsModel)
}

// makeVersions builds a versions slice of length n where the given indices
// are marked as installed ("v1.0") and the rest are empty.
func makeVersions(n int, installed ...int) []string {
	v := make([]string, n)
	for _, i := range installed {
		v[i] = "v1.0"
	}
	return v
}

// ── toolDetectMsg ────────────────────────────────────────────────────────────

func TestToolsModel_toolDetectMsg_SetsLoaded(t *testing.T) {
	m := NewToolsModel(system.Info{})
	if m.loaded {
		t.Fatal("model should not be loaded before receiving toolDetectMsg")
	}
	updated, _ := m.Update(toolDetectMsg{versions: make([]string, len(m.tools))})
	got := updated.(*ToolsModel)
	if !got.loaded {
		t.Error("model.loaded should be true after toolDetectMsg")
	}
}

func TestToolsModel_toolDetectMsg_StoresVersions(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := make([]string, n)
	versions[0] = "git version 2.43.0"
	got := newLoadedModel(versions)

	if got.versions[0] != "git version 2.43.0" {
		t.Errorf("versions[0] = %q; want %q", got.versions[0], "git version 2.43.0")
	}
	for i := 1; i < n; i++ {
		if got.versions[i] != "" {
			t.Errorf("versions[%d] = %q; want empty", i, got.versions[i])
		}
	}
}

func TestToolsModel_toolDetectMsg_ShortVersionsSlice_NoPanic(t *testing.T) {
	m := NewToolsModel(system.Info{})
	if len(m.tools) == 0 {
		t.Skip("no tools in registry")
	}
	short := []string{"v1.0"}
	updated, _ := m.Update(toolDetectMsg{versions: short})
	got := updated.(*ToolsModel)
	if !got.loaded {
		t.Error("model.loaded should be true after toolDetectMsg")
	}
	if got.versions[0] != "v1.0" {
		t.Errorf("versions[0] = %q; want %q", got.versions[0], "v1.0")
	}
	for i := 1; i < len(got.tools); i++ {
		if got.versions[i] != "" {
			t.Errorf("versions[%d] = %q; want empty (not provided)", i, got.versions[i])
		}
	}
}

// ── keyboard navigation ───────────────────────────────────────────────────────

func TestToolsModel_KeyboardBeforeLoaded_IsIgnored(t *testing.T) {
	m := NewToolsModel(system.Info{})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(*ToolsModel)
	if got.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (keyboard ignored before loaded)", got.cursor)
	}
}

func TestToolsModel_Navigation_DownUp(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m = newLoadedModel(makeVersions(len(m.tools)))
	if len(m.displayOrder) < 2 {
		t.Skip("need at least 2 tools")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(*ToolsModel)
	if got.cursor != 1 {
		t.Errorf("cursor after Down = %d; want 1", got.cursor)
	}
	updated2, _ := got.Update(tea.KeyMsg{Type: tea.KeyUp})
	got2 := updated2.(*ToolsModel)
	if got2.cursor != 0 {
		t.Errorf("cursor after Up = %d; want 0", got2.cursor)
	}
}

// ── isAvailable ──────────────────────────────────────────────────────────────

func TestIsAvailable_NoRequires(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools))
	got := newLoadedModel(versions)

	for dp, item := range got.displayOrder {
		if len(got.tools[item.idx].Requires) == 0 {
			if !got.isAvailable(dp) {
				t.Errorf("tool %q (no requires) should be available", got.tools[item.idx].Name)
			}
		}
	}
}

func TestIsAvailable_RequiresNotInstalled(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools))
	got := newLoadedModel(versions)

	for dp, item := range got.displayOrder {
		tool := got.tools[item.idx]
		if len(tool.Requires) > 0 {
			if got.isAvailable(dp) {
				t.Errorf("tool %q should not be available when its requires are not installed", tool.Name)
			}
		}
	}
}

func TestIsAvailable_RequiresInstalled(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n)

	brewIdx := -1
	for i, tool := range m.tools {
		if tool.Name == "brew" {
			brewIdx = i
			break
		}
	}
	if brewIdx < 0 {
		t.Skip("brew not in registry")
	}

	versions[brewIdx] = "Homebrew 4.0.0"
	got := newLoadedModel(versions)

	for dp, item := range got.displayOrder {
		tool := got.tools[item.idx]
		for _, req := range tool.Requires {
			if req == "brew" {
				if !got.isAvailable(dp) {
					t.Errorf("tool %q should be available when brew is installed", tool.Name)
				}
			}
		}
	}
}

// ── tree / display order ──────────────────────────────────────────────────────

func TestBuildDisplayOrder_RootsAppearBeforeChildren(t *testing.T) {
	m := NewToolsModel(system.Info{})
	brewIdx, lazyIdx := -1, -1
	for i, t2 := range m.tools {
		switch t2.Name {
		case "brew":
			brewIdx = i
		case "lazydocker":
			lazyIdx = i
		}
	}
	if brewIdx < 0 || lazyIdx < 0 {
		t.Skip("brew or lazydocker not found in registry")
	}

	brewDP, lazyDP := -1, -1
	for dp, item := range m.displayOrder {
		if item.idx == brewIdx {
			brewDP = dp
		}
		if item.idx == lazyIdx {
			lazyDP = dp
		}
	}
	if brewDP < 0 || lazyDP < 0 {
		t.Fatal("brew or lazydocker not present in displayOrder")
	}
	if brewDP >= lazyDP {
		t.Errorf("brew at display position %d should come before lazydocker at %d", brewDP, lazyDP)
	}
}

func TestBuildDisplayOrder_ChildHasGreaterDepth(t *testing.T) {
	m := NewToolsModel(system.Info{})
	for _, item := range m.displayOrder {
		tool := m.tools[item.idx]
		if len(tool.Requires) > 0 && item.depth == 0 {
			t.Errorf("tool %q has requires but depth=0 in displayOrder", tool.Name)
		}
	}
}

func TestBuildDisplayOrder_CycleNoPanic(t *testing.T) {
	tools := []struct {
		name     string
		requires []string
	}{
		{"a", []string{"b"}},
		{"b", []string{"a"}},
	}
	regs := make([]registry.Tool, len(tools))
	for i, tc := range tools {
		regs[i] = registry.Tool{Name: tc.name, Description: tc.name, Requires: tc.requires}
	}
	_ = buildDisplayOrder(regs)
}

func TestBuildDisplayOrder_EachToolAppearsOnce(t *testing.T) {
	m := NewToolsModel(system.Info{})
	seen := make(map[int]int)
	for _, item := range m.displayOrder {
		seen[item.idx]++
	}
	for idx, count := range seen {
		if count > 1 {
			t.Errorf("tool %q appears %d times in displayOrder; want exactly 1", m.tools[idx].Name, count)
		}
	}
}

// ── treePrefix ───────────────────────────────────────────────────────────────

func TestTreePrefix_Root(t *testing.T) {
	item := toolItem{idx: 0, depth: 0, isLast: false}
	if got := treePrefix(item); got != "" {
		t.Errorf("treePrefix(root) = %q; want empty string", got)
	}
}

func TestTreePrefix_FirstLevelNotLast(t *testing.T) {
	item := toolItem{idx: 0, depth: 1, isLast: false, parentContinues: nil}
	got := treePrefix(item)
	if !strings.Contains(got, "├") {
		t.Errorf("treePrefix(depth=1, notLast) = %q; want to contain ├", got)
	}
}

func TestTreePrefix_FirstLevelLast(t *testing.T) {
	item := toolItem{idx: 0, depth: 1, isLast: true, parentContinues: nil}
	got := treePrefix(item)
	if !strings.Contains(got, "└") {
		t.Errorf("treePrefix(depth=1, isLast) = %q; want to contain └", got)
	}
}

// ── isBusy ───────────────────────────────────────────────────────────────────

func TestIsBusy_IdleWhenNoOpAndEmptyQueue(t *testing.T) {
	m := NewToolsModel(system.Info{})
	if m.isBusy() {
		t.Error("isBusy() should be false on fresh model")
	}
}

func TestIsBusy_TrueWhenActiveNameSet(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.activeName = "git"
	if !m.isBusy() {
		t.Error("isBusy() should be true when activeName is set")
	}
}

func TestIsBusy_TrueWhenQueueNonEmpty(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.opQueue = []pendingOp{{tool: registry.Tool{Name: "git"}}}
	if !m.isBusy() {
		t.Error("isBusy() should be true when opQueue is non-empty")
	}
}

// ── collectInstallOrder ───────────────────────────────────────────────────────

func TestCollectInstallOrder_NoDeps(t *testing.T) {
	m := NewToolsModel(system.Info{})
	gitIdx := -1
	for i, t2 := range m.tools {
		if t2.Name == "git" {
			gitIdx = i
			break
		}
	}
	if gitIdx < 0 {
		t.Skip("git not in registry")
	}
	ops := m.collectInstallOrder(gitIdx, make(map[int]bool))
	if len(ops) != 1 || ops[0].tool.Name != "git" {
		t.Errorf("collectInstallOrder(git) = %v; want single git op", ops)
	}
}

func TestCollectInstallOrder_BrewBeforeLazydocker(t *testing.T) {
	m := NewToolsModel(system.Info{})
	lazyIdx, brewIdx := -1, -1
	for i, t2 := range m.tools {
		switch t2.Name {
		case "lazydocker":
			lazyIdx = i
		case "brew":
			brewIdx = i
		}
	}
	if lazyIdx < 0 || brewIdx < 0 {
		t.Skip("brew or lazydocker not in registry")
	}

	ops := m.collectInstallOrder(lazyIdx, make(map[int]bool))
	if len(ops) < 2 {
		t.Fatalf("collectInstallOrder(lazydocker) = %v; want at least 2 ops (brew + lazydocker)", ops)
	}
	if ops[0].tool.Name != "brew" {
		t.Errorf("first op = %q; want brew", ops[0].tool.Name)
	}
	if ops[len(ops)-1].tool.Name != "lazydocker" {
		t.Errorf("last op = %q; want lazydocker", ops[len(ops)-1].tool.Name)
	}
}

func TestCollectInstallOrder_SkipsInstalled(t *testing.T) {
	m := NewToolsModel(system.Info{})
	lazyIdx, brewIdx := -1, -1
	for i, t2 := range m.tools {
		switch t2.Name {
		case "lazydocker":
			lazyIdx = i
		case "brew":
			brewIdx = i
		}
	}
	if lazyIdx < 0 || brewIdx < 0 {
		t.Skip("brew or lazydocker not in registry")
	}
	// Mark brew as already installed.
	m.versions[brewIdx] = "Homebrew 4.0.0"

	ops := m.collectInstallOrder(lazyIdx, make(map[int]bool))
	for _, op := range ops {
		if op.tool.Name == "brew" {
			t.Error("collectInstallOrder should skip brew when already installed")
		}
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func TestToolsModel_View_ShowsLoadingBeforeDetect(t *testing.T) {
	m := NewToolsModel(system.Info{})
	view := m.View()
	if !strings.Contains(view, "Detecting installed tools") {
		t.Errorf("View() before detection = %q; want it to contain \"Detecting installed tools\"", view)
	}
}

func TestToolsModel_View_ShowsListAfterDetect(t *testing.T) {
	m := NewToolsModel(system.Info{})
	got := newLoadedModel(makeVersions(len(m.tools)))
	view := got.View()
	if strings.Contains(view, "Detecting installed tools") {
		t.Error("View() after detection should not show loading message")
	}
}

func TestToolsModel_View_ShowsVersionInline(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools))
	versions[0] = "git version 2.43.0"
	got := newLoadedModel(versions)
	view := got.View()
	if !strings.Contains(view, "git version 2.43.0") {
		t.Errorf("View() should contain the detected version string, got:\n%s", view)
	}
}

func TestToolsModel_View_ShowsTreeConnector(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools))
	got := newLoadedModel(versions)
	view := got.View()
	if !strings.Contains(view, "├") && !strings.Contains(view, "└") {
		t.Errorf("View() should contain tree connectors (├ or └), got:\n%s", view)
	}
}

func TestToolsModel_View_ShowsRequiresHint(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools))
	// With nothing installed, tools with requires should show as unavailable.
	// The view should still render without panicking.
	got := newLoadedModel(versions)
	_ = got.View()
}

// ── popup ────────────────────────────────────────────────────────────────────

func TestPopup_OpensForInstalledTool(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n)
	versions[0] = "v1.0"
	got := newLoadedModel(versions)

	// Navigate to first tool (which is installed) and press Enter.
	updated, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	result := updated.(*ToolsModel)

	if !result.popupMode {
		t.Error("popupMode should be true after pressing Enter on an installed tool")
	}
}

func TestPopup_EscCancels(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools))
	versions[0] = "v1.0"
	got := newLoadedModel(versions)
	got.popupMode = true
	got.popupToolDP = 0

	updated, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	result := updated.(*ToolsModel)
	if result.popupMode {
		t.Error("popupMode should be false after ESC")
	}
}

// ── CanSwitchTabs ─────────────────────────────────────────────────────────────

func TestCanSwitchTabs_TrueWhenIdle(t *testing.T) {
	m := NewToolsModel(system.Info{})
	if !m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be true when idle")
	}
}

func TestCanSwitchTabs_FalseWhenPopupOpen(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.popupMode = true
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false when popup is open")
	}
}

func TestCanSwitchTabs_FalseWhenPTYFocused(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.ptyFocused = true
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false when PTY is focused")
	}
}

func TestCanSwitchTabs_FalseWhenBusy(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.activeName = "git"
	if m.CanSwitchTabs() {
		t.Error("CanSwitchTabs() should be false while an op is active")
	}
}

// ── CanQuit ───────────────────────────────────────────────────────────────────

func TestCanQuit_TrueWhenIdle(t *testing.T) {
	m := NewToolsModel(system.Info{})
	if !m.CanQuit() {
		t.Error("CanQuit() should be true when idle")
	}
}

func TestCanQuit_FalseWhenPTYFocused(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.ptyFocused = true
	if m.CanQuit() {
		t.Error("CanQuit() should be false when PTY is focused")
	}
}

func TestCanQuit_FalseWhenBusy(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.activeName = "git"
	if m.CanQuit() {
		t.Error("CanQuit() should be false while an op is active")
	}
}

// ── installedDependentsOf transitive ─────────────────────────────────────────

func TestInstalledDependentsOf_TransitiveClosure(t *testing.T) {
	// Build a model with tools a -> b -> c (a requires b, b requires c).
	tools := []registry.Tool{
		{Name: "a", Description: "a", Requires: []string{"b"}},
		{Name: "b", Description: "b", Requires: []string{"c"}},
		{Name: "c", Description: "c"},
	}
	nameToIdx := map[string]int{"a": 0, "b": 1, "c": 2}
	m := &ToolsModel{
		tools:     tools,
		nameToIdx: nameToIdx,
		versions:  []string{"v1", "v1", "v1"}, // all installed
	}
	// Uninstalling c should include transitive dependent a first, then b.
	deps := m.installedDependentsOf(2) // index of c
	if len(deps) != 2 {
		t.Fatalf("installedDependentsOf(c) = %v; want [a b]", deps)
	}
	if deps[0] != "a" || deps[1] != "b" {
		t.Errorf("installedDependentsOf(c) = %v; want [a b]", deps)
	}
}

// ── running phase ─────────────────────────────────────────────────────────────

// runningModel returns a ToolsModel in active-op state with n synthetic ops in
// the queue so tests can send result messages without starting real processes.
func runningModel(n int) *ToolsModel {
	m := NewToolsModel(system.Info{})
	m.loaded = true
	if n > 0 {
		ops := make([]pendingOp, n-1)
		for i := range ops {
			ops[i] = pendingOp{tool: registry.Tool{Name: fmt.Sprintf("tool%d", i+1)}, isUninstall: false}
		}
		m.activeName = "tool0"
		m.opQueue = ops
	}
	return m
}

func TestToolsModel_Running_InstallResult_AdvancesQueue(t *testing.T) {
	m := runningModel(1) // 1 active op, queue empty
	updated, _ := m.Update(installResultMsg{name: "tool0"})
	got := updated.(*ToolsModel)

	if got.activeName != "" {
		t.Errorf("activeName = %q; want empty after last op completes", got.activeName)
	}
	if got.isBusy() {
		t.Error("isBusy() should be false after all ops complete")
	}
}

func TestToolsModel_Running_InstallResult_MarksFailure(t *testing.T) {
	m := runningModel(1)
	updated, _ := m.Update(installResultMsg{name: "tool0", err: fmt.Errorf("install failed")})
	got := updated.(*ToolsModel)

	if len(got.completedOps) == 0 {
		t.Fatal("completedOps should have an entry after a failed install")
	}
	if got.completedOps[0].success {
		t.Error("completedOps[0].success should be false for a failed install")
	}
}

func TestToolsModel_Running_UninstallResult_RecordsSuccess(t *testing.T) {
	m := runningModel(1)
	m.activeIsUninstall = true
	updated, _ := m.Update(uninstallResultMsg{name: "tool0"})
	got := updated.(*ToolsModel)

	if len(got.completedOps) == 0 {
		t.Fatal("completedOps should have an entry after uninstall")
	}
	if !got.completedOps[0].isUninstall {
		t.Error("completedOps[0].isUninstall should be true")
	}
}

func TestToolsModel_Running_WindowSizeMsg_UpdatesWidth(t *testing.T) {
	m := runningModel(1)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(*ToolsModel)
	if got.width != 120 {
		t.Errorf("width = %d; want 120 after WindowSizeMsg", got.width)
	}
}

func TestToolsModel_Running_LogLine_Appended(t *testing.T) {
	m := runningModel(1)
	ch := make(chan string, 1)
	updated, _ := m.Update(logLineMsg{toolName: "tool0", line: "hello", ch: ch})
	got := updated.(*ToolsModel)
	if len(got.toolLogs["tool0"]) == 0 || got.toolLogs["tool0"][0] != "hello" {
		t.Errorf("toolLogs[tool0] = %v; want [\"hello\"]", got.toolLogs["tool0"])
	}
}

// ── pane height calculations ─────────────────────────────────────────────────

func TestComputePaneHeight_AccountsForAppUI(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.height = 50
	got := m.computePaneHeight()
	// No PTY active, so only appUIReservedRows (5) is subtracted.
	want := 45
	if got != want {
		t.Errorf("computePaneHeight() = %d; want %d (height - appUI)", got, want)
	}
}

func TestComputePaneHeight_AccountsForHintWhenPTYActive(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.height = 50
	m.ptyFocused = true
	got := m.computePaneHeight()
	// PTY focused → splitViewHintRows (1) also subtracted: 50 - 5 - 1 = 44.
	want := 44
	if got != want {
		t.Errorf("computePaneHeight() with PTY = %d; want %d (height - appUI - hint)", got, want)
	}
}

func TestComputePaneHeight_ClampsToMinimum(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.height = 3 // very small terminal
	got := m.computePaneHeight()
	// Should clamp to minimum of 5 to avoid negative/zero values
	if got < 5 {
		t.Errorf("computePaneHeight() = %d; want minimum 5", got)
	}
}

func TestComputeVisibleLogLines_SubtractsBordersAndTitle(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.height = 50
	got := m.computeVisibleLogLines()
	// Pane height = 45 (50 - appUIReservedRows 5, no PTY so no hint rows)
	// Content = 40 (45 - paneBorderRows 2 - paneTitleRows 3)
	want := 40
	if got != want {
		t.Errorf("computeVisibleLogLines() = %d; want %d", got, want)
	}
}

func TestComputeVisibleLogLines_EnsuresMinimum(t *testing.T) {
	m := NewToolsModel(system.Info{})
	m.height = 5 // very small terminal
	got := m.computeVisibleLogLines()
	// Should ensure at least 1 line is visible
	if got < 1 {
		t.Errorf("computeVisibleLogLines() = %d; want at least 1", got)
	}
}
