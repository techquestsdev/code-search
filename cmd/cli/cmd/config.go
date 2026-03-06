package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration",
	RunE:  runConfigInit,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE:  runConfigShow,
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configSetCmd)

	configInitCmd.Flags().Bool("local", false, "Create in current directory")
	configInitCmd.Flags().Bool("force", false, "Overwrite existing config")
}

const defaultConfig = `# code-search configuration
api_url: http://localhost:8080
output: text

search:
  limit: 100
  context_lines: 2

replace:
  dry_run: true
  parallel: 5
`

func runConfigInit(cmd *cobra.Command, args []string) error {
	local, _ := cmd.Flags().GetBool("local")
	force, _ := cmd.Flags().GetBool("force")

	var configPath string
	if local {
		configPath = ".code-search.yaml"
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}

		configPath = filepath.Join(home, ".code-search.yaml")
	}

	if _, err := os.Stat(configPath); err == nil && !force {
		return fmt.Errorf("config file already exists: %s (use --force to overwrite)", configPath)
	}

	err := os.WriteFile(configPath, []byte(defaultConfig), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Printf("✓ Created config file: %s\n", configPath)

	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	fmt.Println("Current configuration:")

	if viper.ConfigFileUsed() != "" {
		fmt.Printf("Config file: %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Println("Config file: (none)")
	}

	for _, key := range viper.AllKeys() {
		fmt.Printf("%s: %v\n", key, viper.Get(key))
	}

	return nil
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	viper.Set(key, value)

	if viper.ConfigFileUsed() != "" {
		err := viper.WriteConfig()
		if err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}

		fmt.Printf("✓ Set %s = %s\n", key, value)
	} else {
		return errors.New("no config file found, run 'code-search config init' first")
	}

	return nil
}
