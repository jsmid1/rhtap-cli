package config

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/redhat-appstudio/rhtap-cli/pkg/chartfs"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v3"
)

// ErrInvalidConfig indicates the configuration content is invalid.
var ErrInvalidConfig = errors.New("invalid configuration")

// ErrEmptyConfig indicates the configuration file is empty.
var ErrEmptyConfig = errors.New("empty configuration")

// ErrUnmarshalConfig indicates the configuration file structure is invalid.
var ErrUnmarshalConfig = errors.New("failed to unmarshal configuration")

// Spec contains all configuration sections.
type Spec struct {
	// Namespace installer's namespace, where the installer's resources will be
	// deployed. Note, Helm charts deployed by the installer are likely to use a
	// different namespace.
	Namespace string `yaml:"namespace"`
	// Features contains the configuration for the installer features.
	Features map[string]FeatureSpec `yaml:"features"`
	// Dependencies contains the installer Helm chart dependencies.
	Dependencies []Dependency `yaml:"dependencies"`
}

// Config root configuration structure.
type Config struct {
	cfs        *chartfs.ChartFS // embedded filesystem
	configPath string           // configuration file path

	Installer Spec `yaml:"rhtapCLI"` // root configuration for the installer
}

// PersistentFlags defines the persistent flags for the CLI.
func (c *Config) PersistentFlags(f *pflag.FlagSet) {
	f.StringVar(
		&c.configPath,
		"config",
		c.configPath,
		"Path to the installer configuration file",
	)
}

// GetBaseDir returns the base directory of the configuration file.
func (c *Config) GetBaseDir() string {
	return filepath.Dir(c.configPath)
}

// GetDependency returns a dependency chart configuration.
func (c *Config) GetDependency(logger *slog.Logger, chart string) (*Dependency, error) {
	logger.Debug("Getting dependency")
	for _, dep := range c.Installer.Dependencies {
		if dep.Chart == chart {
			return &dep, nil
		}
	}
	return nil, fmt.Errorf("chart %s not found", chart)
}

// GetEnabledDependencies returns a list of enabled dependencies.
func (c *Config) GetEnabledDependencies(logger *slog.Logger) []Dependency {
	enabled := []Dependency{}
	logger.Debug("Getting enabled dependencies")
	for _, dep := range c.Installer.Dependencies {
		if dep.Enabled {
			logger.Debug("Using dependency...", "dep-chart", dep.Chart)
			enabled = append(enabled, dep)
		} else {
			logger.Debug("Skipping dependency...", "dep-chart", dep.Chart)
		}
	}
	return enabled
}

// GetFeature returns a feature by name, or an error if the feature is not found.
func (c *Config) GetFeature(name string) (*FeatureSpec, error) {
	feature, ok := c.Installer.Features[name]
	if !ok {
		return nil, fmt.Errorf("feature %s not found", name)
	}
	return &feature, nil
}

// Validate validates the configuration, checking for missing fields.
func (c *Config) Validate() error {
	root := c.Installer
	// The installer itself must have a namespace.
	if root.Namespace == "" {
		return fmt.Errorf("%w: missing namespace", ErrInvalidConfig)
	}

	// Validating the features, making sure every feature entry is valid.
	for _, feature := range root.Features {
		if err := feature.Validate(); err != nil {
			return err
		}
	}

	// Making sure the installer has a list of dependencies.
	if len(root.Dependencies) == 0 {
		return fmt.Errorf("%w: missing dependencies", ErrInvalidConfig)
	}
	// Validating each dependency, making sure they have the required fields.
	for pos, dep := range root.Dependencies {
		if dep.Chart == "" {
			return fmt.Errorf(
				"%w: missing chart in dependency %d", ErrInvalidConfig, pos)
		}
		if dep.Namespace == "" {
			return fmt.Errorf(
				"%w: missing namespace in dependency %d", ErrInvalidConfig, pos)
		}
	}
	return nil
}

// UnmarshalYAML reads the configuration file and unmarshal it into the Config.
func (c *Config) UnmarshalYAML() error {
	payload, err := c.cfs.ReadFile(c.configPath)
	if err != nil {
		return err
	}
	if len(payload) == 0 {
		return fmt.Errorf("%w: %s", ErrEmptyConfig, c.configPath)
	}
	if err = yaml.Unmarshal(payload, c); err != nil {
		return fmt.Errorf("%w: %s %w", ErrUnmarshalConfig, c.configPath, err)
	}
	return c.Validate()
}

// NewConfigFromFile returns a new Config instance based on the informed file. The
// config file path is kept as a private attribute.
func NewConfigFromFile(cfs *chartfs.ChartFS, configPath string) (*Config, error) {
	c := NewConfig(cfs)
	c.configPath = configPath
	return c, c.UnmarshalYAML()
}

// NewConfig returns a new Config instance, pointing to the default "config.yaml"
// file location.
func NewConfig(cfs *chartfs.ChartFS) *Config {
	return &Config{
		configPath: "installer/config.yaml",
		cfs:        cfs,
	}
}
