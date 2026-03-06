package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/techquestsdev/code-search/cmd/cli/client"
)

var (
	cfgFile     string
	apiURL      string
	iapClientID string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "code-search",
	Short: "A fast code search and replace tool",
	Long: `code-search is a CLI tool for searching and replacing code across 
multiple repositories. It supports creating merge requests for changes
and integrates with GitHub and GitLab.

Examples:
  # Search for a pattern
  code-search search "func main"

  # Search with filters
  code-search search "FOO" --repos myorg/myrepo --langs go

  # Find files matching a pattern
  code-search find files "*.go"

  # Replace across repos (creates MR)
  code-search replace "oldFunc" "newFunc" --execute`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().
		StringVar(&cfgFile, "config", "", "config file (default is $HOME/.code-search.yaml)")
	rootCmd.PersistentFlags().
		StringVar(&apiURL, "api-url", "http://localhost:8080", "API server URL")
	rootCmd.PersistentFlags().String("output", "text", "Output format: text, json, table")
	rootCmd.PersistentFlags().String("token", "", "Code host token for replace operations (GitHub/GitLab PAT)")
	rootCmd.PersistentFlags().String("auth-token", "", "Authentication token (JWT or API token, env: CODE_SEARCH_AUTH_TOKEN)")
	rootCmd.PersistentFlags().
		StringVar(&iapClientID, "iap-client-id", "", "IAP OAuth client ID (for servers behind Google IAP)")

	// Bind to viper
	viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("output", rootCmd.PersistentFlags().Lookup("output"))
	viper.BindPFlag("token", rootCmd.PersistentFlags().Lookup("token"))
	viper.BindPFlag("auth_token", rootCmd.PersistentFlags().Lookup("auth-token"))
	viper.BindPFlag("iap_client_id", rootCmd.PersistentFlags().Lookup("iap-client-id"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.AddConfigPath(".")
		viper.SetConfigName(".code-search")
		viper.SetConfigType("yaml")
	}

	viper.SetEnvPrefix("CODE_SEARCH")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

// newClient creates a new API client with authentication support.
func newClient() *client.Client {
	apiURL := viper.GetString("api_url")
	token := viper.GetString("token")
	authToken := viper.GetString("auth_token")
	iapClientID := viper.GetString("iap_client_id")

	var opts []client.ClientOption
	if authToken != "" {
		opts = append(opts, client.WithAuthToken(authToken))
	}

	if iapClientID != "" {
		opts = append(opts, client.WithIAPClientID(iapClientID))
	}

	return client.New(apiURL, token, opts...)
}
