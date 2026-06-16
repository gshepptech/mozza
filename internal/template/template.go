package template

import (
	"fmt"
	"strings"
)

// Template represents a pre-built deployable app template.
type Template struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	Icon         string        `json:"icon"`
	Category     string        `json:"category"`
	Tags         []string      `json:"tags"`
	Source       string        `json:"source"`
	Variables    []TemplateVar `json:"variables"`
	Repo         string        `json:"repo,omitempty"`
	Official     bool          `json:"official"`
	MinK8sVer    string        `json:"min_k8s_ver,omitempty"`
	EstResources string        `json:"est_resources,omitempty"`
}

// TemplateVar is a user-configurable setting in a template.
type TemplateVar struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Type        string   `json:"type"`
	Default     string   `json:"default"`
	Required    bool     `json:"required"`
	Options     []string `json:"options,omitempty"`
}

// RenderRecipe replaces {{VAR}} placeholders with provided values.
// Returns an error if a required variable is missing.
func RenderRecipe(tmpl Template, vars map[string]string) (string, error) {
	result := tmpl.Source

	for _, v := range tmpl.Variables {
		val, provided := vars[v.Key]
		if !provided {
			if v.Required {
				return "", fmt.Errorf("required variable %q is missing", v.Key)
			}
			val = v.Default
		}
		result = strings.ReplaceAll(result, "{{"+v.Key+"}}", val)
	}

	return result, nil
}

// RenderDefaults replaces {{VAR}} placeholders that have defaults, leaving
// required variables without defaults as-is for the user to fill in.
func RenderDefaults(tmpl Template) string {
	result := tmpl.Source
	for _, v := range tmpl.Variables {
		if v.Default != "" {
			result = strings.ReplaceAll(result, "{{"+v.Key+"}}", v.Default)
		}
	}
	return result
}
