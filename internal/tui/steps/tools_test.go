package steps

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/system"
)

// newLoadedModel returns a ToolsModel that has already received a toolDetectMsg
// with the given versions slice (one entry per tool from registry.List()).
func newLoadedModel(versions []string) *ToolsModel {
	m := NewToolsModel(system.Info{OS: "linux", PackageManager: "apt"})
	updated, _ := m.Update(toolDetectMsg{versions: versions})
	return updated.(*ToolsModel)
}

// makeVersions builds a slice of length n where indices in installed are
// set to "v1.0" and the rest are empty.
func makeVersions(n int, installed ...int) []string {
	v := make([]string, n)
	for _, i := range installed {
		v[i] = "v1.0"
	}
	return v
}

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

func TestToolsModel_toolDetectMsg_PreChecksInstalledTools(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	if n < 2 {
		t.Skip("need at least 2 tools in registry for this test")
	}

	// Mark only the first two tools as installed.
	versions := makeVersions(n, 0, 1)
	got := newLoadedModel(versions)

	for i := 0; i < n; i++ {
		want := i == 0 || i == 1
		if got.checked[i] != want {
			t.Errorf("checked[%d] = %v; want %v (tool %q)", i, got.checked[i], want, got.tools[i].Name)
		}
	}
}

func TestToolsModel_toolDetectMsg_AllUninstalled(t *testing.T) {
	m := NewToolsModel(system.Info{})
	versions := makeVersions(len(m.tools)) // all empty
	got := newLoadedModel(versions)

	for i, c := range got.checked {
		if c {
			t.Errorf("checked[%d] = true; want false (tool %q not installed)", i, got.tools[i].Name)
		}
	}
}

func TestToolsModel_toolDetectMsg_AllInstalled(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}
	versions := makeVersions(n, indices...)
	got := newLoadedModel(versions)

	for i, c := range got.checked {
		if !c {
			t.Errorf("checked[%d] = false; want true (tool %q installed)", i, got.tools[i].Name)
		}
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
	if !strings.Contains(view, "Select tools to install") {
		t.Errorf("View() after detection = %q; want it to contain \"Select tools to install\"", view)
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

func TestToolsModel_KeyboardBeforeLoaded_IsIgnored(t *testing.T) {
	m := NewToolsModel(system.Info{})
	// cursor should not move before loaded
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	got := updated.(*ToolsModel)
	if got.cursor != 0 {
		t.Errorf("cursor = %d; want 0 (keyboard ignored before loaded)", got.cursor)
	}
}

func TestToolsModel_toolDetectMsg_ShortVersionsSlice_NoPanic(t *testing.T) {
	m := NewToolsModel(system.Info{})
	if len(m.tools) == 0 {
		t.Skip("no tools in registry")
	}
	// Send a versions slice shorter than the number of tools; should not panic
	// and should only populate the entries that were provided.
	short := []string{"v1.0"} // length 1, regardless of how many tools there are
	updated, _ := m.Update(toolDetectMsg{versions: short})
	got := updated.(*ToolsModel)
	if !got.loaded {
		t.Error("model.loaded should be true after toolDetectMsg")
	}
	if got.versions[0] != "v1.0" {
		t.Errorf("versions[0] = %q; want %q", got.versions[0], "v1.0")
	}
	// Entries beyond the provided slice must remain at their zero value.
	for i := 1; i < len(got.tools); i++ {
		if got.versions[i] != "" {
			t.Errorf("versions[%d] = %q; want empty (not provided)", i, got.versions[i])
		}
	}
}

// ── tree / dependency helpers ────────────────────────────────────────────────

func TestBuildDisplayOrder_RootsAppearBeforeChildren(t *testing.T) {
	m := NewToolsModel(system.Info{})
	brewIdx := -1
	lazyIdx := -1
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

	// Find their display positions.
	brewDP := -1
	lazyDP := -1
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
		if len(tool.Requires) > 0 {
			if item.depth == 0 {
				t.Errorf("tool %q has requires but depth=0 in displayOrder", tool.Name)
			}
		}
	}
}

func TestBuildDisplayOrder_CycleNoPanic(t *testing.T) {
	// A synthetic cyclic graph: A requires B, B requires A.
	// buildDisplayOrder must not hang or panic.
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
	// Should not hang.
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

func TestIsAvailable_NoRequires(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n) // nothing installed
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
	n := len(m.tools)
	versions := makeVersions(n) // nothing installed
	got := newLoadedModel(versions)

	for dp, item := range got.displayOrder {
		tool := got.tools[item.idx]
		if len(tool.Requires) > 0 {
			if got.isAvailable(dp) {
				t.Errorf("tool %q should not be available when its requires are not installed/checked", tool.Name)
			}
		}
	}
}

func TestIsAvailable_RequiresInstalledViaVersion(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n)

	// Find brew by name rather than by a hardcoded index.
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

	// Mark brew as installed.
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

func TestCascadeUncheck_UnchecksDependent(t *testing.T) {
	// Scenario: brew is NOT installed (no version), user checks brew then lazydocker.
	// When brew is unchecked again, lazydocker must be auto-unchecked because its
	// dependency (brew) is no longer installed or selected.
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n) // nothing installed
	got := newLoadedModel(versions)

	// Find brew and lazydocker display positions.
	brewDP := -1
	lazyDP := -1
	for dp, item := range got.displayOrder {
		switch got.tools[item.idx].Name {
		case "brew":
			brewDP = dp
		case "lazydocker":
			lazyDP = dp
		}
	}
	if brewDP < 0 || lazyDP < 0 {
		t.Skip("brew or lazydocker not in registry")
	}

	// Check brew first so lazydocker becomes available.
	got.setChecked(brewDP, true)
	if !got.isAvailable(lazyDP) {
		t.Fatal("lazydocker should be available once brew is checked")
	}

	// Check lazydocker.
	got.setChecked(lazyDP, true)

	// Uncheck brew — cascade must propagate and uncheck lazydocker.
	got.setChecked(brewDP, false)

	if got.checked[got.displayOrder[lazyDP].idx] {
		t.Error("lazydocker should have been auto-unchecked after brew was unchecked")
	}
}

func TestToolsModel_View_ShowsRequiresHint(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n) // nothing installed -> lazydocker is disabled
	got := newLoadedModel(versions)

	view := got.View()
	if !strings.Contains(view, "requires:") {
		t.Errorf("View() should show a 'requires:' hint for disabled tools, got:\n%s", view)
	}
}

func TestToolsModel_View_ShowsTreeConnector(t *testing.T) {
	m := NewToolsModel(system.Info{})
	n := len(m.tools)
	versions := makeVersions(n)
	got := newLoadedModel(versions)

	view := got.View()
	// The tree connector ├ or └ should appear for lazydocker.
	if !strings.Contains(view, "├") && !strings.Contains(view, "└") {
		t.Errorf("View() should contain tree connectors (├ or └), got:\n%s", view)
	}
}

// ── uninstall feature ────────────────────────────────────────────────────────

// newLoadedModelWithVersions returns a loaded model where the named tools are
// treated as already installed (version = "v1.0").
func newLoadedModelWithVersions(installedNames ...string) *ToolsModel {
	m := NewToolsModel(system.Info{OS: "linux", PackageManager: "apt"})
	nameSet := make(map[string]bool, len(installedNames))
	for _, n := range installedNames {
		nameSet[n] = true
	}
	versions := make([]string, len(m.tools))
	for i, t := range m.tools {
		if nameSet[t.Name] {
			versions[i] = "v1.0"
		}
	}
	updated, _ := m.Update(toolDetectMsg{versions: versions})
	return updated.(*ToolsModel)
}

func TestApplyConfirmedRemoval_MarksForUninstall(t *testing.T) {
	m := newLoadedModelWithVersions("git")

	gitDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "git" {
			gitDP = dp
			break
		}
	}
	if gitDP < 0 {
		t.Skip("git not in registry")
	}

	if !m.checked[m.displayOrder[gitDP].idx] {
		t.Fatal("git should be checked after detection as installed")
	}

	m.applyConfirmedRemoval(gitDP)

	idx := m.displayOrder[gitDP].idx
	if m.checked[idx] {
		t.Error("git should be unchecked after applyConfirmedRemoval")
	}
	if !m.toUninstall[idx] {
		t.Error("git should be marked toUninstall after applyConfirmedRemoval")
	}
}

func TestApplyConfirmedRemoval_CascadesToInstalledDependents(t *testing.T) {
	// brew and lazydocker are both "installed".
	m := newLoadedModelWithVersions("brew", "lazydocker")

	brewDP, lazyDP := -1, -1
	for dp, item := range m.displayOrder {
		switch m.tools[item.idx].Name {
		case "brew":
			brewDP = dp
		case "lazydocker":
			lazyDP = dp
		}
	}
	if brewDP < 0 || lazyDP < 0 {
		t.Skip("brew or lazydocker not in registry")
	}

	m.applyConfirmedRemoval(brewDP)

	brewIdx := m.displayOrder[brewDP].idx
	lazyIdx := m.displayOrder[lazyDP].idx

	if !m.toUninstall[brewIdx] {
		t.Error("brew should be marked toUninstall")
	}
	if !m.toUninstall[lazyIdx] {
		t.Error("lazydocker should be marked toUninstall (installed dependent of brew)")
	}
	if m.checked[lazyIdx] {
		t.Error("lazydocker should be unchecked")
	}
}

func TestCheckedDependentsOf_ReturnsDepNames(t *testing.T) {
	// brew is installed and checked; lazydocker is also checked.
	m := newLoadedModelWithVersions("brew", "lazydocker")

	brewIdx := -1
	for i, t2 := range m.tools {
		if t2.Name == "brew" {
			brewIdx = i
			break
		}
	}
	if brewIdx < 0 {
		t.Skip("brew not in registry")
	}

	deps := m.checkedDependentsOf(brewIdx)
	found := false
	for _, d := range deps {
		if d == "lazydocker" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("checkedDependentsOf(brew) = %v; want to contain lazydocker", deps)
	}
}

func TestCheckedDependentsOf_EmptyWhenNoDependents(t *testing.T) {
	m := newLoadedModelWithVersions("git")

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

	deps := m.checkedDependentsOf(gitIdx)
	if len(deps) != 0 {
		t.Errorf("checkedDependentsOf(git) = %v; want empty (no tools require git)", deps)
	}
}

func TestSetChecked_ClearsToUninstall(t *testing.T) {
	m := newLoadedModelWithVersions("git")

	gitDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "git" {
			gitDP = dp
			break
		}
	}
	if gitDP < 0 {
		t.Skip("git not in registry")
	}

	// First confirm removal to set toUninstall.
	m.applyConfirmedRemoval(gitDP)
	idx := m.displayOrder[gitDP].idx
	if !m.toUninstall[idx] {
		t.Fatal("toUninstall should be true after applyConfirmedRemoval")
	}

	// Re-check — toUninstall should be cleared.
	m.setChecked(gitDP, true)
	if m.toUninstall[idx] {
		t.Error("toUninstall should be false after re-checking the tool")
	}
	if !m.checked[idx] {
		t.Error("checked should be true after setChecked(true)")
	}
}

func TestToggleOrConfirm_OpensPopupForInstalledTool(t *testing.T) {
	m := newLoadedModelWithVersions("git")

	gitDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "git" {
			gitDP = dp
			break
		}
	}
	if gitDP < 0 {
		t.Skip("git not in registry")
	}

	m.cursor = gitDP
	m.toggleOrConfirm(gitDP)

	if !m.popupMode {
		t.Error("popupMode should be true after toggleOrConfirm on an installed tool")
	}
	if m.popupToolDP != gitDP {
		t.Errorf("popupToolDP = %d; want %d", m.popupToolDP, gitDP)
	}
}

func TestToggleOrConfirm_DirectToggleForUncheckedTool(t *testing.T) {
	// Nothing installed; toggle an unchecked tool directly (no popup).
	m := newLoadedModelWithVersions()

	// Find a root tool with no requires so it's available.
	targetDP := -1
	for dp, item := range m.displayOrder {
		if len(m.tools[item.idx].Requires) == 0 {
			targetDP = dp
			break
		}
	}
	if targetDP < 0 {
		t.Skip("no root tool found in registry")
	}

	m.toggleOrConfirm(targetDP)

	if m.popupMode {
		t.Error("popupMode should not be set for a tool that is not installed")
	}
	if !m.checked[m.displayOrder[targetDP].idx] {
		t.Error("tool should be checked after direct toggle")
	}
}

func TestPopupMode_EscCancelsAndKeepsChecked(t *testing.T) {
	m := newLoadedModelWithVersions("git")

	gitDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "git" {
			gitDP = dp
			break
		}
	}
	if gitDP < 0 {
		t.Skip("git not in registry")
	}

	m.cursor = gitDP
	m.toggleOrConfirm(gitDP)
	if !m.popupMode {
		t.Fatal("expected popupMode to be open")
	}

	// Press ESC — popup should close without changes.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*ToolsModel)

	if got.popupMode {
		t.Error("popupMode should be false after ESC")
	}
	if !got.checked[got.displayOrder[gitDP].idx] {
		t.Error("git should still be checked after ESC (removal cancelled)")
	}
}

func TestView_ShowsPopupWhenInPopupMode(t *testing.T) {
	m := newLoadedModelWithVersions("git")

	gitDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "git" {
			gitDP = dp
			break
		}
	}
	if gitDP < 0 {
		t.Skip("git not in registry")
	}

	m.toggleOrConfirm(gitDP)
	view := m.View()

	if !strings.Contains(view, "Remove tool") {
		t.Errorf("View() in popup mode should contain 'Remove tool', got:\n%s", view)
	}
	if !strings.Contains(view, "Yes, Remove") {
		t.Errorf("View() in popup mode should contain 'Yes, Remove', got:\n%s", view)
	}
}

func TestView_ShowsPendingRemovalBadge(t *testing.T) {
	m := newLoadedModelWithVersions("git")

	gitDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "git" {
			gitDP = dp
			break
		}
	}
	if gitDP < 0 {
		t.Skip("git not in registry")
	}

	m.applyConfirmedRemoval(gitDP)
	view := m.View()

	if !strings.Contains(view, "✗") {
		t.Errorf("View() should show ✗ badge for tool pending removal, got:\n%s", view)
	}
}

func TestIsAvailable_UnavailableWhenDepPendingUninstall(t *testing.T) {
	// brew is installed, lazydocker is installed; confirm removal of brew.
	// lazydocker should now be unavailable because its required tool is pending uninstall.
	m := newLoadedModelWithVersions("brew", "lazydocker")

	brewDP, lazyDP := -1, -1
	for dp, item := range m.displayOrder {
		switch m.tools[item.idx].Name {
		case "brew":
			brewDP = dp
		case "lazydocker":
			lazyDP = dp
		}
	}
	if brewDP < 0 || lazyDP < 0 {
		t.Skip("brew or lazydocker not in registry")
	}

	// Before removal: lazydocker should be available.
	if !m.isAvailable(lazyDP) {
		t.Fatal("lazydocker should be available before any removal")
	}

	// Mark brew for uninstall without the full applyConfirmedRemoval cascade.
	brewIdx := m.displayOrder[brewDP].idx
	m.toUninstall[brewIdx] = true
	m.checked[brewIdx] = false

	// Now lazydocker must be unavailable.
	if m.isAvailable(lazyDP) {
		t.Error("lazydocker should be unavailable when its dep (brew) is pending uninstall")
	}
}

func TestIsAvailable_PreventsRecheckAfterDepRemoval(t *testing.T) {
	// Confirm removal of brew, then lazydocker must not be selectable.
	m := newLoadedModelWithVersions("brew", "lazydocker")

	brewDP, lazyDP := -1, -1
	for dp, item := range m.displayOrder {
		switch m.tools[item.idx].Name {
		case "brew":
			brewDP = dp
		case "lazydocker":
			lazyDP = dp
		}
	}
	if brewDP < 0 || lazyDP < 0 {
		t.Skip("brew or lazydocker not in registry")
	}

	m.applyConfirmedRemoval(brewDP)

	if m.isAvailable(lazyDP) {
		t.Error("lazydocker should not be available after brew is confirmed for removal")
	}
}

func TestToggleOrConfirm_SeparatesInstalledAndDeselectedDeps(t *testing.T) {
	// brew is installed, lazydocker is installed, another tool that requires brew but is
	// only selected (not installed yet) would go to deselectedDeps.
	// For registry tools: brew=installed, lazydocker=installed.
	// popupDeps should contain "lazydocker" (installed), popupDeselectedDeps should be empty.
	m := newLoadedModelWithVersions("brew", "lazydocker")

	brewDP := -1
	for dp, item := range m.displayOrder {
		if m.tools[item.idx].Name == "brew" {
			brewDP = dp
			break
		}
	}
	if brewDP < 0 {
		t.Skip("brew not in registry")
	}

	m.toggleOrConfirm(brewDP)
	if !m.popupMode {
		t.Fatal("popupMode should be open")
	}

	// lazydocker is installed → must be in popupDeps (will be uninstalled)
	foundInstalled := false
	for _, d := range m.popupDeps {
		if d == "lazydocker" {
			foundInstalled = true
			break
		}
	}
	if !foundInstalled {
		t.Errorf("popupDeps = %v; want to contain lazydocker (installed)", m.popupDeps)
	}

	// lazydocker must NOT appear in popupDeselectedDeps
	for _, d := range m.popupDeselectedDeps {
		if d == "lazydocker" {
			t.Errorf("popupDeselectedDeps = %v; should not contain lazydocker (it is installed)", m.popupDeselectedDeps)
		}
	}
}

func TestToggleOrConfirm_SelectedOnlyDepGoesToDeselectedList(t *testing.T) {
	// brew installed, lazydocker only selected (version == "").
	// popupDeps should be empty, popupDeselectedDeps should contain lazydocker.
	m := newLoadedModelWithVersions("brew") // only brew is installed

	// Manually check lazydocker (it becomes available once brew is installed/checked).
	lazyDP := -1
	brewDP := -1
	for dp, item := range m.displayOrder {
		switch m.tools[item.idx].Name {
		case "lazydocker":
			lazyDP = dp
		case "brew":
			brewDP = dp
		}
	}
	if lazyDP < 0 || brewDP < 0 {
		t.Skip("brew or lazydocker not in registry")
	}

	// lazydocker should now be available (brew installed) and unchecked; check it.
	if !m.isAvailable(lazyDP) {
		t.Fatal("lazydocker should be available once brew is installed")
	}
	m.setChecked(lazyDP, true)

	// Open popup for brew removal.
	m.toggleOrConfirm(brewDP)
	if !m.popupMode {
		t.Fatal("popupMode should be open")
	}

	// lazydocker is not installed → must be in popupDeselectedDeps
	foundDeselected := false
	for _, d := range m.popupDeselectedDeps {
		if d == "lazydocker" {
			foundDeselected = true
			break
		}
	}
	if !foundDeselected {
		t.Errorf("popupDeselectedDeps = %v; want to contain lazydocker (selected but not installed)", m.popupDeselectedDeps)
	}

	// lazydocker must NOT appear in popupDeps
	for _, d := range m.popupDeps {
		if d == "lazydocker" {
			t.Errorf("popupDeps = %v; should not contain lazydocker (it is not installed)", m.popupDeps)
		}
	}
}

// ── running-phase: Ctrl+C abort flow ─────────────────────────────────────────

// runningModel returns a ToolsModel placed directly in the running state with
// n synthetic pending operations, so tests can send messages without starting
// real goroutines.
func runningModel(n int) *ToolsModel {
	m := NewToolsModel(system.Info{})
	ops := make([]pendingOp, n)
	for i := range ops {
		ops[i] = pendingOp{tool: registry.Tool{Name: fmt.Sprintf("tool%d", i)}, isUninstall: false}
	}
	m.pendingOps = ops
	m.opIdx = 0
	m.opSuccess = make([]bool, n)
	m.toolLogs = make(map[string][]string)
	m.currentTool = ops[0].tool.Name
	m.running = true
	return m
}

func TestToolsModel_Running_CtrlC_EntersAbortMode(t *testing.T) {
	m := runningModel(1)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	got := updated.(*ToolsModel)
	if !got.abortMode {
		t.Error("abortMode should be true after ctrl+c while running")
	}
	if !got.running {
		t.Error("running should still be true when abort mode is entered")
	}
}

func TestToolsModel_Running_AbortConfirm_YesTransitionsToDone(t *testing.T) {
	m := runningModel(1)
	m.abortMode = true
	m.abortCursor = 0 // "Yes, Abort"

	cancelled := false
	m.cancelFn = func() { cancelled = true }

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*ToolsModel)

	if !got.done {
		t.Error("done should be true after confirming abort")
	}
	if got.running {
		t.Error("running should be false after confirming abort")
	}
	if !cancelled {
		t.Error("cancelFn should have been called on abort confirmation")
	}
	found := false
	for _, e := range got.errors {
		if strings.Contains(e, "aborted") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("errors should contain an 'aborted' message; got %v", got.errors)
	}
}

func TestToolsModel_Running_AbortConfirm_EscDismisses(t *testing.T) {
	m := runningModel(1)
	m.abortMode = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(*ToolsModel)

	if got.abortMode {
		t.Error("abortMode should be false after ESC")
	}
	if !got.running {
		t.Error("running should remain true after ESC")
	}
}

func TestToolsModel_Running_AbortConfirm_CancelKeepsRunning(t *testing.T) {
	m := runningModel(1)
	m.abortMode = true
	m.abortCursor = 1 // "Continue"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(*ToolsModel)

	if got.done {
		t.Error("done should be false after choosing Continue in abort dialog")
	}
	if !got.running {
		t.Error("running should remain true after choosing Continue in abort dialog")
	}
}

// ── running-phase: sequential op driver ──────────────────────────────────────

func TestToolsModel_Running_InstallResult_AdvancesOpIdx(t *testing.T) {
	m := runningModel(2)
	m.currentTool = "tool0"

	// Simulate successful install of tool0. opIdx advances 0→1; startNextOp
	// launches tool1 (which would start a real goroutine), so we use a single-
	// op model instead to keep the test synchronous.
	m2 := runningModel(1) // 1 op: after advancing, startNextOp returns nil (done)
	updated, _ := m2.Update(installResultMsg{name: "tool0"})
	got := updated.(*ToolsModel)

	if got.opIdx != 1 {
		t.Errorf("opIdx = %d; want 1 after installResultMsg", got.opIdx)
	}
	if !got.opSuccess[0] {
		t.Error("opSuccess[0] should be true for a successful install")
	}
	if !got.done {
		t.Error("done should be true once all ops complete")
	}
}

func TestToolsModel_Running_InstallResult_MarksFailure(t *testing.T) {
	m := runningModel(1)
	updated, _ := m.Update(installResultMsg{name: "tool0", err: fmt.Errorf("install failed")})
	got := updated.(*ToolsModel)

	if got.opSuccess[0] {
		t.Error("opSuccess[0] should be false for a failed install")
	}
	if len(got.errors) == 0 {
		t.Error("errors should contain an entry for the failed install")
	}
}

func TestToolsModel_Running_UninstallResult_AdvancesOpIdx(t *testing.T) {
	m := runningModel(1)
	m.pendingOps[0].isUninstall = true

	updated, _ := m.Update(uninstallResultMsg{name: "tool0"})
	got := updated.(*ToolsModel)

	if got.opIdx != 1 {
		t.Errorf("opIdx = %d; want 1 after uninstallResultMsg", got.opIdx)
	}
	if !got.opSuccess[0] {
		t.Error("opSuccess[0] should be true for a successful uninstall")
	}
}

func TestToolsModel_Running_LogLineMsg_AppendsCappedAtMax(t *testing.T) {
	m := runningModel(1)
	m.currentTool = "tool0"

	// Fill past the cap.
	for i := 0; i < logMaxLines+5; i++ {
		ch := make(chan string)
		close(ch) // closed channel → waitForLog returns logDoneMsg next tick
		updated, _ := m.Update(logLineMsg{toolName: "tool0", line: fmt.Sprintf("line%d", i), ch: ch})
		m = updated.(*ToolsModel)
	}

	stored := m.toolLogs["tool0"]
	if len(stored) > logMaxLines {
		t.Errorf("toolLogs[tool0] has %d entries; want at most %d", len(stored), logMaxLines)
	}
}

func TestToolsModel_Running_WindowSizeMsg_UpdatesWidth(t *testing.T) {
	m := runningModel(1)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	got := updated.(*ToolsModel)

	if got.width != 120 {
		t.Errorf("width = %d; want 120 after WindowSizeMsg while running", got.width)
	}
}

