package registry

import (
	"testing"
)

func TestList_ReturnsAllTools(t *testing.T) {
	got := List()
	if len(got) == 0 {
		t.Fatal("List() returned empty slice; expected at least one tool")
	}
}

func TestList_ReturnsCopy(t *testing.T) {
	a := List()
	b := List()
	a[0].Name = "mutated"
	if b[0].Name == "mutated" {
		t.Error("List() returned a shared slice; mutations must not affect subsequent calls")
	}
}

func TestList_AllToolsHaveRequiredFields(t *testing.T) {
	for _, tool := range List() {
		if tool.Name == "" {
			t.Errorf("tool with empty Name found: %+v", tool)
		}
		if tool.Description == "" {
			t.Errorf("tool %q has empty Description", tool.Name)
		}
	}
}

func TestFind_KnownTool(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"git"},
		{"neovim"},
		{"docker"},
		{"zsh"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Find(tc.name)
			if got == nil {
				t.Fatalf("Find(%q) = nil; want non-nil", tc.name)
			}
			if got.Name != tc.name {
				t.Errorf("Find(%q).Name = %q; want %q", tc.name, got.Name, tc.name)
			}
		})
	}
}

func TestFind_UnknownTool(t *testing.T) {
	got := Find("nonexistent-tool-xyz")
	if got != nil {
		t.Errorf("Find(unknown) = %+v; want nil", got)
	}
}

func TestFind_CaseSensitive(t *testing.T) {
	got := Find("Git")
	if got != nil {
		t.Error("Find() should be case-sensitive; 'Git' must not match 'git'")
	}
}

func TestList_ContainsExpectedTools(t *testing.T) {
	expected := []string{"git", "neovim", "docker", "nodejs", "python3", "curl", "tmux", "htop", "ripgrep", "fzf", "zsh", "starship"}
	index := make(map[string]bool, len(List()))
	for _, tool := range List() {
		index[tool.Name] = true
	}
	for _, name := range expected {
		if !index[name] {
			t.Errorf("expected tool %q not found in registry", name)
		}
	}
}
