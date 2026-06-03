// Package config manages debi CLI configuration: a TOML file holding
// non-secret per-profile settings. Secrets (API keys) are never stored here;
// they live in the OS keychain (see pkg/keyring) or the DEBI_API_KEY env var.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

const (
	// DefaultProfileName is used when no profile is explicitly selected.
	DefaultProfileName = "default"

	// EnvAPIKey overrides any stored key. Useful for CI and containers.
	EnvAPIKey = "DEBI_API_KEY"

	// EnvProfile selects the active profile.
	EnvProfile = "DEBI_PROFILE"

	// ModeTest and ModeLive identify the API environment.
	ModeTest = "test"
	ModeLive = "live"
)

// Config is the in-memory representation of the CLI configuration.
type Config struct {
	// Profile is the name of the currently active profile.
	Profile string

	// File is the absolute path to the config file backing this Config.
	File string

	v *viper.Viper
}

// Profile holds the non-secret settings for a single named profile.
type Profile struct {
	Name       string
	Mode       string
	APIVersion string
	DeviceName string
}

// New loads the configuration from disk, creating directories as needed.
// configFile may be empty to use the default location.
func New(configFile, profile string) (*Config, error) {
	if configFile == "" {
		dir, err := Dir()
		if err != nil {
			return nil, err
		}
		configFile = filepath.Join(dir, "config.toml")
	}

	v := viper.New()
	v.SetConfigFile(configFile)
	v.SetConfigType("toml")

	if _, err := os.Stat(configFile); err == nil {
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("reading config %s: %w", configFile, err)
		}
	}

	if profile == "" {
		profile = os.Getenv(EnvProfile)
	}
	if profile == "" {
		profile = v.GetString("profile")
	}
	if profile == "" {
		profile = DefaultProfileName
	}

	return &Config{Profile: profile, File: configFile, v: v}, nil
}

// Dir returns the directory where debi stores its configuration.
func Dir() (string, error) {
	if custom := os.Getenv("DEBI_CONFIG_DIR"); custom != "" {
		return custom, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "debi"), nil
}

// CurrentProfile returns the settings for the active profile, applying
// sensible defaults for unset values.
func (c *Config) CurrentProfile() *Profile {
	return c.GetProfile(c.Profile)
}

// GetProfile returns the settings for the named profile.
func (c *Config) GetProfile(name string) *Profile {
	mode := c.v.GetString(name + ".mode")
	if mode == "" {
		mode = ModeTest
	}
	return &Profile{
		Name:       name,
		Mode:       mode,
		APIVersion: c.v.GetString(name + ".api_version"),
		DeviceName: c.v.GetString(name + ".device_name"),
	}
}

// Profiles returns the names of all configured profiles, sorted.
func (c *Config) Profiles() []string {
	seen := map[string]struct{}{}
	for _, key := range c.v.AllKeys() {
		if i := strings.IndexByte(key, '.'); i > 0 {
			seen[key[:i]] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		if name == "profile" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Set writes a single key for the active profile and persists the file.
func (c *Config) Set(key, value string) error {
	c.v.Set(c.Profile+"."+key, value)
	return c.write()
}

// SetActiveProfile records the default profile and persists the file.
func (c *Config) SetActiveProfile(name string) error {
	c.Profile = name
	c.v.Set("profile", name)
	return c.write()
}

// Unset removes the entire active profile section and persists the file.
// viper has no native delete, so we rebuild the underlying map.
func (c *Config) Unset(name string) error {
	settings := c.v.AllSettings()
	delete(settings, name)
	if settings["profile"] == name {
		delete(settings, "profile")
	}

	nv := viper.New()
	nv.SetConfigFile(c.File)
	nv.SetConfigType("toml")
	for k, val := range settings {
		nv.Set(k, val)
	}
	c.v = nv
	return c.write()
}

func (c *Config) write() error {
	if err := os.MkdirAll(filepath.Dir(c.File), 0o755); err != nil {
		return err
	}
	return c.v.WriteConfigAs(c.File)
}
