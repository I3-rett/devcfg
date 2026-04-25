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
	if len(a) == 0 || len(b) == 0 {
		t.Fatal("List() returned empty slice; cannot test copy semantics")
	}
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

func TestBinaryName(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
		want string
	}{
		{"binary set", Tool{Name: "nvm", Binary: "nvm"}, "nvm"},
		{"binary empty falls back to name", Tool{Name: "git", Binary: ""}, "git"},
		{"binary same as name", Tool{Name: "curl", Binary: "curl"}, "curl"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.tool.BinaryName()
			if got != tc.want {
				t.Errorf("BinaryName() = %q; want %q", got, tc.want)
			}
		})
	}
}

func TestBinaryNames(t *testing.T) {
	tests := []struct {
		name string
		tool Tool
		want []string
	}{
		{"no aliases falls back to BinaryName", Tool{Name: "git", Binary: "git"}, []string{"git"}},
		{"no aliases no binary falls back to name", Tool{Name: "curl"}, []string{"curl"}},
		{"aliases returned as-is", Tool{Name: "bat", Binary: "bat", BinaryAliases: []string{"bat", "batcat"}}, []string{"bat", "batcat"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.tool.BinaryNames()
			if len(got) != len(tc.want) {
				t.Fatalf("BinaryNames() = %v; want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("BinaryNames()[%d] = %q; want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestBat_HasBinaryAliases(t *testing.T) {
	tool := Find("bat")
	if tool == nil {
		t.Fatal("Find(\"bat\") = nil; want non-nil")
	}
	aliases := tool.BinaryNames()
	hasBat := false
	hasBatcat := false
	for _, a := range aliases {
		if a == "bat" {
			hasBat = true
		}
		if a == "batcat" {
			hasBatcat = true
		}
	}
	if !hasBat {
		t.Errorf("bat tool BinaryNames() = %v; want to contain \"bat\"", aliases)
	}
	if !hasBatcat {
		t.Errorf("bat tool BinaryNames() = %v; want to contain \"batcat\"", aliases)
	}
}

func TestList_ContainsExpectedTools(t *testing.T) {
	expected := []string{"brew", "git", "neovim", "docker", "lazydocker", "nvm", "python3", "curl", "tmux", "htop", "ripgrep", "fzf", "zsh", "bat", "font-roboto-mono-nerd-font", "nvchad"}
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

func TestFind_RequiresField(t *testing.T) {
	tests := []struct {
		toolName string
		requires []string
	}{
		{"lazydocker", []string{"brew"}},
		{"git", nil},
		{"brew", nil},
	}
	for _, tc := range tests {
		t.Run(tc.toolName, func(t *testing.T) {
			tool := Find(tc.toolName)
			if tool == nil {
				t.Fatalf("Find(%q) = nil; want non-nil", tc.toolName)
			}
			if len(tool.Requires) != len(tc.requires) {
				t.Errorf("Find(%q).Requires = %v; want %v", tc.toolName, tool.Requires, tc.requires)
				return
			}
			for i, req := range tc.requires {
				if tool.Requires[i] != req {
					t.Errorf("Find(%q).Requires[%d] = %q; want %q", tc.toolName, i, tool.Requires[i], req)
				}
			}
		})
	}
}
