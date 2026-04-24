package resolver

import (
	"testing"

	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/system"
)

func TestResolve_Brew(t *testing.T) {
	tool := registry.Tool{Name: "neovim", Brew: "neovim", Apt: "neovim"}
	sys := system.Info{OS: "macos", PackageManager: "brew"}

	got, err := Resolve(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"brew", "install", "neovim"}
	assertArgs(t, got, want)
}

func TestResolve_Apt(t *testing.T) {
	tool := registry.Tool{Name: "git", Brew: "git", Apt: "git"}
	sys := system.Info{OS: "ubuntu", PackageManager: "apt"}

	got, err := Resolve(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sudo", "apt-get", "install", "-y", "git"}
	assertArgs(t, got, want)
}

func TestResolve_Fallback(t *testing.T) {
	tool := registry.Tool{Name: "starship", Brew: "", Apt: "", Fallback: "curl -sS https://starship.rs/install.sh | sh"}
	sys := system.Info{OS: "linux", PackageManager: "none"}

	got, err := Resolve(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sh", "-c", "curl -sS https://starship.rs/install.sh | sh"}
	assertArgs(t, got, want)
}

func TestResolve_FallbackWhenBrewPkgMissing(t *testing.T) {
	tool := registry.Tool{Name: "starship", Brew: "", Apt: "", Fallback: "sh -c install.sh"}
	sys := system.Info{OS: "macos", PackageManager: "brew"}

	got, err := Resolve(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sh", "-c", "sh -c install.sh"}
	assertArgs(t, got, want)
}

func TestResolve_FallbackWhenAptPkgMissing(t *testing.T) {
	tool := registry.Tool{Name: "starship", Brew: "", Apt: "", Fallback: "sh -c install.sh"}
	sys := system.Info{OS: "ubuntu", PackageManager: "apt"}

	got, err := Resolve(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sh", "-c", "sh -c install.sh"}
	assertArgs(t, got, want)
}

func TestResolve_NoMethodAvailable(t *testing.T) {
	tool := registry.Tool{Name: "mytool", Brew: "", Apt: "", Fallback: ""}
	sys := system.Info{OS: "linux", PackageManager: "none"}

	_, err := Resolve(tool, sys)
	if err == nil {
		t.Fatal("expected error when no install method available, got nil")
	}
}

func TestResolve_NoMethodAvailableErrorMessage(t *testing.T) {
	tool := registry.Tool{Name: "mytool", Brew: "", Apt: "", Fallback: ""}
	sys := system.Info{OS: "freebsd", PackageManager: "none"}

	_, err := Resolve(tool, sys)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got := err.Error(); len(got) == 0 {
		t.Error("error message must not be empty")
	}
}

func TestResolve_AptToolWithBrewPackageManager_UsesFallback(t *testing.T) {
	// brew manager but no brew package defined → should fall through to fallback
	tool := registry.Tool{Name: "aptonly", Brew: "", Apt: "aptonly", Fallback: "install.sh"}
	sys := system.Info{OS: "macos", PackageManager: "brew"}

	got, err := Resolve(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sh", "-c", "install.sh"}
	assertArgs(t, got, want)
}

// assertArgs checks that two string slices are equal.
func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("args length = %d, want %d: got %v, want %v", len(got), len(want), got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// ── ResolveUninstall ─────────────────────────────────────────────────────────

func TestResolveUninstall_Brew(t *testing.T) {
	tool := registry.Tool{Name: "neovim", Brew: "neovim", Apt: "neovim"}
	sys := system.Info{OS: "macos", PackageManager: "brew"}

	got, err := ResolveUninstall(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"brew", "uninstall", "neovim"}
	assertArgs(t, got, want)
}

func TestResolveUninstall_Apt(t *testing.T) {
	tool := registry.Tool{Name: "git", Brew: "git", Apt: "git"}
	sys := system.Info{OS: "ubuntu", PackageManager: "apt"}

	got, err := ResolveUninstall(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"sudo", "apt-get", "remove", "-y", "git"}
	assertArgs(t, got, want)
}

func TestResolveUninstall_NoMethod(t *testing.T) {
	tool := registry.Tool{Name: "mytool", Brew: "", Apt: "", Fallback: "install.sh"}
	sys := system.Info{OS: "linux", PackageManager: "none"}

	_, err := ResolveUninstall(tool, sys)
	if err == nil {
		t.Fatal("expected error when no uninstall method available, got nil")
	}
}

func TestResolveUninstall_BrewWithFullFormula(t *testing.T) {
	tool := registry.Tool{Name: "lazydocker", Brew: "jesseduffield/lazydocker/lazydocker"}
	sys := system.Info{OS: "macos", PackageManager: "brew"}

	got, err := ResolveUninstall(tool, sys)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"brew", "uninstall", "jesseduffield/lazydocker/lazydocker"}
	assertArgs(t, got, want)
}
