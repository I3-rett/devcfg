package system

import (
	"os"
	"path/filepath"
	"testing"
)

// writeOSRelease writes a minimal /etc/os-release substitute to a temp file
// and returns its path.
func writeOSRelease(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "os-release")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write os-release: %v", err)
	}
	return path
}

func TestDetectLinuxDistro_Ubuntu(t *testing.T) {
	path := writeOSRelease(t, "ID=ubuntu\nNAME=\"Ubuntu\"\n")
	got := detectLinuxDistroFromFile(path)
	if got != "ubuntu" {
		t.Errorf("detectLinuxDistroFromFile = %q; want %q", got, "ubuntu")
	}
}

func TestDetectLinuxDistro_Debian(t *testing.T) {
	path := writeOSRelease(t, "ID=debian\nNAME=\"Debian GNU/Linux\"\n")
	got := detectLinuxDistroFromFile(path)
	if got != "debian" {
		t.Errorf("detectLinuxDistroFromFile = %q; want %q", got, "debian")
	}
}

func TestDetectLinuxDistro_UnknownDistro(t *testing.T) {
	path := writeOSRelease(t, "ID=arch\nNAME=\"Arch Linux\"\n")
	got := detectLinuxDistroFromFile(path)
	if got != "linux" {
		t.Errorf("detectLinuxDistroFromFile = %q; want %q", got, "linux")
	}
}

func TestDetectLinuxDistro_QuotedID(t *testing.T) {
	path := writeOSRelease(t, `ID="ubuntu"` + "\n")
	got := detectLinuxDistroFromFile(path)
	if got != "ubuntu" {
		t.Errorf("detectLinuxDistroFromFile (quoted ID) = %q; want %q", got, "ubuntu")
	}
}

func TestDetectLinuxDistro_MissingFile(t *testing.T) {
	got := detectLinuxDistroFromFile("/nonexistent/path/os-release")
	if got != "linux" {
		t.Errorf("detectLinuxDistroFromFile (missing file) = %q; want %q", got, "linux")
	}
}

func TestDetectLinuxDistro_EmptyFile(t *testing.T) {
	path := writeOSRelease(t, "")
	got := detectLinuxDistroFromFile(path)
	if got != "linux" {
		t.Errorf("detectLinuxDistroFromFile (empty file) = %q; want %q", got, "linux")
	}
}

func TestIsInPath_KnownBinary(t *testing.T) {
	// "sh" is universally available on Linux/macOS
	if !isInPath("sh") {
		t.Error("isInPath(\"sh\") = false; want true")
	}
}

func TestIsInPath_UnknownBinary(t *testing.T) {
	if isInPath("_devcfg_nonexistent_binary_xyz") {
		t.Error("isInPath(nonexistent) = true; want false")
	}
}

func TestDetect_ReturnsInfo(t *testing.T) {
	info := Detect()
	if info.OS == "" {
		t.Error("Detect().OS is empty")
	}
	validPMs := map[string]bool{"brew": true, "apt": true, "none": true}
	if !validPMs[info.PackageManager] {
		t.Errorf("Detect().PackageManager = %q; want one of brew/apt/none", info.PackageManager)
	}
}
