package template

import (
	"fmt"
	"strings"
	"text/template"
)

// AnonymousTemplate wraps a *template.Template and implements the
// yaml.Unmarshaler and encoding.TextUnmarshaler interfaces
// so we can unmarshal a template string into a template.
type AnonymousTemplate[T any] struct {
	*template.Template
}

// Execute wraps template.Template.Execute to provide an easier-to-use interface
// and only allow T to be passed as data.
func (t AnonymousTemplate[T]) Execute(data T) (string, error) {
	if t.Template == nil {
		return "", fmt.Errorf("template is nil, trying to execute for data %v", data)
	}
	sb := strings.Builder{}
	if err := t.Template.Execute(&sb, data); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (t *AnonymousTemplate[T]) UnmarshalYAML(unmarshal func(any) error) error {
	var templateString string
	if err := unmarshal(&templateString); err != nil {
		return fmt.Errorf("unmarshal template string: %w", err)
	}
	t.Template = template.New(templateString)
	if _, err := t.Template.Parse(templateString); err != nil {
		return fmt.Errorf("parse template string: %w", err)
	}
	return nil
}

// UnmarshalText implements the encoding.TextUnmarshaler interface.
func (t *AnonymousTemplate[T]) UnmarshalText(text []byte) error {
	t.Template = template.New(string(text))
	if _, err := t.Template.Parse(string(text)); err != nil {
		return fmt.Errorf("parse template string: %w", err)
	}
	return nil
}
