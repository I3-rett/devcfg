package system

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Info struct {
	OS             string // "macos", "ubuntu", "debian", "linux"
	PackageManager string // "brew", "apt", "none"
}

func Detect() Info {
	info := Info{OS: "linux", PackageManager: "none"}

	switch runtime.GOOS {
	case "darwin":
		info.OS = "macos"
		if isInPath("brew") {
			info.PackageManager = "brew"
		}
		return info
	case "linux":
		info.OS = detectLinuxDistro()
	}

	if isInPath("apt-get") || isInPath("apt") {
		info.PackageManager = "apt"
	}

	return info
}

func detectLinuxDistro() string {
	return detectLinuxDistroFromFile("/etc/os-release")
}

func detectLinuxDistroFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "linux"
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			id := strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), `"`))
			switch id {
			case "ubuntu":
				return "ubuntu"
			case "debian":
				return "debian"
			}
			return "linux"
		}
	}
	if err := scanner.Err(); err != nil {
		return "linux"
	}
	return "linux"
}

func isInPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// DetectToolVersion checks whether a binary is in PATH and returns its version
// string (first line of output). Returns an empty string if the binary is not
// found or produces no usable output.
func DetectToolVersion(binary string) string {
	if _, err := exec.LookPath(binary); err != nil {
		return ""
	}
	for _, flag := range []string{"--version", "-V"} {
		// binary comes from the compile-time embedded registry (tools.json), not user input.
		out, err := exec.Command(binary, flag).Output() //nolint:gosec
		if err == nil && len(out) > 0 {
			return strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
		}
	}
	return binary + " (installed)"
}
