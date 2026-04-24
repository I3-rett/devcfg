package resolver

import (
	"errors"

	"github.com/I3-rett/devcfg/internal/registry"
	"github.com/I3-rett/devcfg/internal/system"
)

// Resolve returns the command args to install a tool based on the detected system.
func Resolve(tool registry.Tool, sys system.Info) ([]string, error) {
	switch sys.PackageManager {
	case "brew":
		if tool.Brew != "" {
			return []string{"brew", "install", tool.Brew}, nil
		}
	case "apt":
		if tool.Apt != "" {
			return []string{"sudo", "apt-get", "install", "-y", tool.Apt}, nil
		}
	}

	if tool.Fallback != "" {
		return []string{"sh", "-c", tool.Fallback}, nil
	}

	return nil, errors.New("no installation method available for " + tool.Name + " on " + sys.OS)
}

// ResolveUninstall returns the command args to uninstall a tool based on the detected system.
func ResolveUninstall(tool registry.Tool, sys system.Info) ([]string, error) {
	switch sys.PackageManager {
	case "brew":
		if tool.Brew != "" {
			return []string{"brew", "uninstall", tool.Brew}, nil
		}
	case "apt":
		if tool.Apt != "" {
			return []string{"sudo", "apt-get", "remove", "-y", tool.Apt}, nil
		}
	}
	return nil, errors.New("no uninstall method available for " + tool.Name + " on " + sys.OS)
}
