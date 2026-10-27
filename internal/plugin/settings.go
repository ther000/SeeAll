package plugin

import (
	"strconv"
	"strings"
	"sync"
)

// SettingType is the kind of field a setting shows.
type SettingType string

const (
	// SettingText is a line of text.
	SettingText SettingType = "text"
	// SettingNumber is a number.
	SettingNumber SettingType = "number"
	// SettingBool is a switch.
	SettingBool SettingType = "bool"
	// SettingChoice is one of Options.
	SettingChoice SettingType = "choice"
)

// Option is one value a SettingChoice offers.
type Option struct {
	Value string
	Label string
}

// Setting is one field of an instance's settings.
type Setting struct {
	Key         string
	Label       string
	Description string
	Type        SettingType
	Default     string
	Placeholder string
	Options     []Option
	// OptionsFor fills Options in when the page is drawn, for a list only the
	// running machine knows: the graphics cards it has. It is asked on every
	// frame the settings are built, so it must be cheap or cache its answer.
	OptionsFor func() []Option
	// Secret marks a value the settings page hides, such as a token.
	Secret bool
}

// Settings is one instance's values: the defaults of its manifest's
// settings, changed by the user and kept in the app's settings file.
type Settings struct {
	mu       sync.RWMutex
	defs     []Setting
	values   map[string]string
	onChange func(key, value string)
}

// NewSettings makes settings for a manifest's fields.
func NewSettings(defs []Setting) *Settings {
	values := make(map[string]string, len(defs))
	for _, d := range defs {
		values[d.Key] = d.Default
	}
	return &Settings{defs: defs, values: values, onChange: func(string, string) {}}
}

// Defs returns the fields.
func (s *Settings) Defs() []Setting {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Setting, len(s.defs))
	copy(out, s.defs)
	return out
}

// Get returns a value, or its default when it is unset.
func (s *Settings) Get(key string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.values[key]
}

// Bool reads a switch.
func (s *Settings) Bool(key string, def bool) bool {
	v := strings.TrimSpace(s.Get(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

// Int reads a number.
func (s *Settings) Int(key string, def int) int {
	v := strings.TrimSpace(s.Get(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Float reads a decimal number.
func (s *Settings) Float(key string, def float64) float64 {
	v := strings.TrimSpace(s.Get(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return n
}

// Set changes a value and tells the instance, which may restart its work.
func (s *Settings) Set(key, value string) {
	s.mu.Lock()
	if s.values == nil {
		s.values = map[string]string{}
	}
	old := s.values[key]
	s.values[key] = value
	s.mu.Unlock()
	if old != value {
		s.onChange(key, value)
	}
}

// All returns every value, for saving to the app's settings file.
func (s *Settings) All() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.values))
	for k, v := range s.values {
		out[k] = v
	}
	return out
}

// Load replaces the values with saved ones, keeping defaults for the rest.
func (s *Settings) Load(values map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range values {
		s.values[k] = v
	}
}

// OnChange sets what runs after a value changes, on the goroutine that set
// it.
func (s *Settings) OnChange(fn func(key, value string)) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}
