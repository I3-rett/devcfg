package registry

import (
	_ "embed"
	"encoding/json"
)

//go:embed tools.json
var toolsJSON []byte

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Binary      string `json:"binary"`
	Brew        string `json:"brew"`
	Apt         string `json:"apt"`
	Fallback    string `json:"fallback"`
}

// BinaryName returns the executable name for the tool.
// Falls back to Name when the Binary field is empty.
func (t Tool) BinaryName() string {
	if t.Binary != "" {
		return t.Binary
	}
	return t.Name
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
