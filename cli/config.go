package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// getConfigDir returns the path to the solvr config directory (~/.solvr)
func getConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".solvr")
}

// getConfigPath returns the path to the solvr config file (~/.solvr/config)
func getConfigPath() string {
	return filepath.Join(getConfigDir(), "config")
}

// loadConfig reads the config file and returns a map of key-value pairs
func loadConfig() (map[string]string, error) {
	config := make(map[string]string)
	configPath := getConfigPath()

	file, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil // Return empty config if file doesn't exist
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			config[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return config, nil
}

// saveConfig writes the config map to the config file
func saveConfig(config map[string]string) error {
	configDir := getConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := getConfigPath()
	file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	for key, value := range config {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// maskAPIKey masks an API key for display, showing only first and last few chars
func maskAPIKey(key string) string {
	if len(key) <= 10 {
		return "****"
	}
	return key[:6] + "****" + key[len(key)-4:]
}

// NewConfigCmd creates the config command
func NewConfigCmd() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage Solvr CLI configuration",
		Long: `Manage Solvr CLI configuration.

Configuration is stored in ~/.solvr/config.

Available subcommands:
  set  - Set a configuration value
  get  - Get configuration values`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	configCmd.AddCommand(NewConfigSetCmd())
	configCmd.AddCommand(NewConfigGetCmd())

	return configCmd
}

// NewConfigSetCmd creates the config set subcommand
func NewConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value.

Examples:
  solvr config set api-key solvr_xxx
  solvr config set api-url https://api.solvr.dev`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			value := args[1]

			config, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			config[key] = value

			if err := saveConfig(config); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Configuration set: %s\n", key)
			return nil
		},
	}
}

// NewConfigGetCmd creates the config get subcommand
func NewConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Get configuration values",
		Long: `Get configuration values.

If a key is specified, shows only that value.
If no key is specified, shows all configuration.

Examples:
  solvr config get           # Show all config
  solvr config get api-url   # Show specific value`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			config, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// If a specific key is requested
			if len(args) == 1 {
				key := args[0]
				value, exists := config[key]
				if !exists {
					fmt.Fprintf(cmd.OutOrStdout(), "Key '%s' not found in configuration\n", key)
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), value)
				return nil
			}

			// Show all config
			if len(config) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No configuration found. Use 'solvr config set' to configure.")
				return nil
			}

			for key, value := range config {
				displayValue := value
				// Mask sensitive values
				if key == "api-key" {
					displayValue = maskAPIKey(value)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, displayValue)
			}

			return nil
		},
	}
}
