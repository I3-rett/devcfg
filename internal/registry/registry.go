package registry

import (
	_ "embed"
	"encoding/json"
)

//go:embed tools.json
var toolsJSON []byte

type Tool struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Binary         string   `json:"binary"`
	BinaryAliases  []string `json:"binaryAliases,omitempty"`
	Brew           string   `json:"brew"`
	Apt            string   `json:"apt"`
	Fallback       string   `json:"fallback"`
	Requires       []string `json:"requires,omitempty"`
}

// BinaryName returns the primary executable name for the tool.
// Falls back to Name when the Binary field is empty.
func (t Tool) BinaryName() string {
	if t.Binary != "" {
		return t.Binary
	}
	return t.Name
}

// BinaryNames returns all candidate executable names for the tool.
// When BinaryAliases is non-empty it is returned as-is; otherwise a
// single-element slice containing BinaryName() is returned.  Callers
// that need to detect whether a tool is installed should try each name
// in order and stop at the first match.
func (t Tool) BinaryNames() []string {
	if len(t.BinaryAliases) > 0 {
		return t.BinaryAliases
	}
	return []string{t.BinaryName()}
}

var tools []Tool

func init() {
	if err := json.Unmarshal(toolsJSON, &tools); err != nil {
		panic("failed to parse tools.json: " + err.Error())
	}
}

func List() []Tool {
	out := make([]Tool, len(tools))
	copy(out, tools)
	return out
}

func Find(name string) *Tool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}
