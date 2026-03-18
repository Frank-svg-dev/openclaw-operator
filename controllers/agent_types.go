package controllers

type AgentDefaultsConfig struct {
	Agents map[string]interface{} `json:"agents"`
}

type DefaultsConfig struct {
	Model     ModelConfig `json:"model"`
	Workspace string      `json:"workspace"`
}

type ModelConfig struct {
	Primary string `json:"primary"`
}

type AgentListItem struct {
	ID        string       `json:"id"`
	Name      string       `json:"name,omitempty"`
	Workspace string       `json:"workspace,omitempty"`
	Default   bool         `json:"default,omitempty"`
	Model     *ModelConfig `json:"model,omitempty"`
}
