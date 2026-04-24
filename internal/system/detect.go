package system

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
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

// DetectToolVersion checks whether a binary is in PATH and returns the first
// non-empty line of its combined (stdout+stderr) output from "--version" or
// "-V". A 5-second context timeout is applied to each probe so the UI never
// hangs on a blocking binary. Returns an empty string if the binary is not
// found. If the binary exists but produces no usable version output, it
// returns "<binary> (installed)" as a fallback.
func DetectToolVersion(binary string) string {
	if _, err := exec.LookPath(binary); err != nil {
		return ""
	}
	for _, flag := range []string{"--version", "-V"} {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		// binary comes from the compile-time embedded registry (tools.json), not user input.
		out, err := exec.CommandContext(ctx, binary, flag).CombinedOutput() //nolint:gosec
		cancel()
		if err != nil {
			// Allow ExitError (non-zero exit): the output may still contain a
			// useful version string. Abort on any other error (timeout, etc.).
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				continue
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return binary + " (installed)"
}
