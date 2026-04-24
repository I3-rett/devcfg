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
	Brew        string `json:"brew"`
	Apt         string `json:"apt"`
	Fallback    string `json:"fallback"`
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
