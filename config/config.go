package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all gtc settings, populated from ~/.gtc.yaml or env vars.
type Config struct {
	Token   string   `mapstructure:"token"`
	BaseURL string   `mapstructure:"base_url"`
	Repos   []string `mapstructure:"repos"`
	Output  string   `mapstructure:"output"` // table | json | yaml
}

// Load reads configuration from (in priority order):
//  1. Environment variables  GITLAB_TOKEN, GTC_BASE_URL …
//  2. ~/.gtc.yaml
//  3. Built-in defaults
func Load() (*Config, error) {
	viper.SetConfigName(".gtc")
	viper.SetConfigType("yaml")

	// Search in home dir first, then current dir
	if home, err := os.UserHomeDir(); err == nil {
		viper.AddConfigPath(home)
	}
	viper.AddConfigPath(".")

	// Defaults
	viper.SetDefault("base_url", "https://gitlab.com")
	viper.SetDefault("output", "table")

	// Env var bindings  (e.g. GITLAB_TOKEN overrides token)
	viper.SetEnvPrefix("GTC")
	viper.AutomaticEnv()
	_ = viper.BindEnv("token", "GITLAB_TOKEN")

	// Read config file — not an error if absent
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// ConfigFilePath returns the default config file path for the init command.
func ConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gtc.yaml"), nil
}
