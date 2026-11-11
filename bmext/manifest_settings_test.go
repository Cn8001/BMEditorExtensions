package bmext

import (
	"errors"
	"testing"
)

func TestValidate_SettingOptionsAndWhen(t *testing.T) {
	// Arrange
	base := func(settings ...Setting) Manifest {
		return Manifest{Name: "ac", Version: "0.1.0", Settings: settings}
	}
	provider := Setting{Key: "provider", Label: "Server", Options: []Option{{Value: "local"}, {Value: "ollama"}}}
	cases := []struct {
		name string
		m    Manifest
		ok   bool
	}{
		{"options and when", base(provider, Setting{Key: "url", Label: "URL", When: map[string]string{"provider": "ollama"}}), true},
		{"options on a boolean", base(Setting{Key: "gpu", Label: "GPU", Type: "boolean", Options: []Option{{Value: "x"}}}), false},
		{"an option without a value", base(Setting{Key: "m", Label: "M", Options: []Option{{Label: "none"}}}), false},
		{"when names no setting", base(Setting{Key: "url", Label: "URL", When: map[string]string{"server": "ollama"}}), false},
		{"when names itself", base(Setting{Key: "url", Label: "URL", When: map[string]string{"url": "x"}}), false},
	}
	for _, c := range cases {
		// Act
		err := c.m.Validate()

		// Assert
		if (err == nil) != c.ok || (err != nil && !errors.Is(err, ErrInvalid)) {
			t.Errorf("%s: %v", c.name, err)
		}
	}
}
