package steps

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
