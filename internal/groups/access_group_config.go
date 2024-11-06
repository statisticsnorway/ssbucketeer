package groups

import (
	"fmt"
	"strings"
	"text/template"
	"time"
)

type AccessConfig struct {
	Name            string          `yaml:"name"`
	ProjectTemplate projectTemplate `yaml:"projectTemplate"`
	MaxDuration     time.Duration   `yaml:"maxDuration"`
	ReasonRequired  bool            `yaml:"reasonRequired"`
}

type ProjectTemplateData struct {
	TeamName string
	Stage    string
}

// projectTemplate wraps a *template.Template and implements yaml.Unmarshaler interface
// so we can unmarshal a template string into a template.
type projectTemplate struct {
	template *template.Template
}

func (t projectTemplate) Name() string {
	if t.template == nil {
		return "<nil>"
	}
	return t.template.Name()
}

// Execute wraps template.Template.Execute to provide an easier-to-use interface
// and only allow ProjectTemplateData to be passed as data.
func (t projectTemplate) Execute(data ProjectTemplateData) (string, error) {
	if t.template == nil {
		return "", fmt.Errorf("template is nil, trying to execute for data %q", data)
	}
	sb := strings.Builder{}
	if err := t.template.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (t *projectTemplate) UnmarshalYAML(unmarshal func(any) error) error {
	var templateString string
	if err := unmarshal(&templateString); err != nil {
		return fmt.Errorf("unmarshal template string: %w", err)
	}
	t.template = template.New(templateString)
	if _, err := t.template.Parse(templateString); err != nil {
		return fmt.Errorf("parse template string: %w", err)
	}
	return nil
}
