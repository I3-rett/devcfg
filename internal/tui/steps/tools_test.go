package steps

import (
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
