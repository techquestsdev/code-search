package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/techquestsdev/code-search/cmd/cli/client"
)

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find files, symbols, or references",
}

var findFilesCmd = &cobra.Command{
	Use:   "files <pattern>",
	Short: "Find files matching a pattern",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindFiles,
}

var findSymbolsCmd = &cobra.Command{
	Use:   "symbols <name>",
	Short: "Find symbol definitions",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindSymbols,
}

var findRefsCmd = &cobra.Command{
	Use:   "refs <symbol>",
	Short: "Find references to a symbol",
	Args:  cobra.ExactArgs(1),
	RunE:  runFindRefs,
}

func init() {
	rootCmd.AddCommand(findCmd)
	findCmd.AddCommand(findFilesCmd)
	findCmd.AddCommand(findSymbolsCmd)
	findCmd.AddCommand(findRefsCmd)

	findFilesCmd.Flags().BoolP("regex", "r", false, "Treat pattern as regex")
	findFilesCmd.Flags().StringSliceP("repos", "R", nil, "Filter by repositories")
	findFilesCmd.Flags().IntP("limit", "n", 100, "Maximum number of results")

	findSymbolsCmd.Flags().StringP("type", "t", "", "Symbol type (function, class, etc.)")
	findSymbolsCmd.Flags().StringP("lang", "l", "", "Filter by language")
	findSymbolsCmd.Flags().StringSliceP("repos", "R", nil, "Filter by repositories")
	findSymbolsCmd.Flags().IntP("limit", "n", 100, "Maximum number of results")

	findRefsCmd.Flags().StringSliceP("repos", "R", nil, "Filter by repositories")
	findRefsCmd.Flags().StringP("lang", "l", "", "Filter by language")
	findRefsCmd.Flags().IntP("limit", "n", 100, "Maximum number of results")
}

func runFindFiles(cmd *cobra.Command, args []string) error {
	pattern := args[0]
	output := viper.GetString("output")

	isRegex, _ := cmd.Flags().GetBool("regex")
	repos, _ := cmd.Flags().GetStringSlice("repos")
	limit, _ := cmd.Flags().GetInt("limit")

	// Build repo filter (first repo if specified)
	repo := ""
	if len(repos) > 0 {
		repo = repos[0]
	}

	c := newClient()

	resp, err := c.FindFiles(context.Background(), &client.FindFilesRequest{
		Pattern: pattern,
		IsRegex: isRegex,
		Repo:    repo,
		Limit:   limit,
	})
	if err != nil {
		return fmt.Errorf("find files failed: %w", err)
	}

	switch output {
	case "json":
		return outputJSON(resp)
	default:
		return outputFilesText(resp)
	}
}

func outputFilesText(resp *client.FindFilesResponse) error {
	if len(resp.Files) == 0 {
		fmt.Println("No files found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "REPOSITORY\tFILE\n")

	for _, f := range resp.Files {
		fmt.Fprintf(w, "%s\t%s\n", f.Repository, f.FilePath)
	}

	w.Flush()

	fmt.Printf("\n%d files found\n", resp.Total)

	return nil
}

func runFindSymbols(cmd *cobra.Command, args []string) error {
	name := args[0]
	output := viper.GetString("output")

	kind, _ := cmd.Flags().GetString("type")
	lang, _ := cmd.Flags().GetString("lang")
	repos, _ := cmd.Flags().GetStringSlice("repos")
	limit, _ := cmd.Flags().GetInt("limit")

	c := newClient()

	symbols, err := c.FindSymbols(context.Background(), &client.FindSymbolsRequest{
		Name:     name,
		Kind:     kind,
		Language: lang,
		Repos:    repos,
		Limit:    limit,
	})
	if err != nil {
		return fmt.Errorf("find symbols failed: %w", err)
	}

	switch output {
	case "json":
		return outputJSON(symbols)
	default:
		return outputSymbolsText(symbols)
	}
}

func outputSymbolsText(symbols []client.Symbol) error {
	if len(symbols) == 0 {
		fmt.Println("No symbols found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tKIND\tFILE\tLINE\tREPO\n")

	for _, s := range symbols {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", s.Name, s.Kind, s.File, s.Line, s.Repo)
	}

	w.Flush()

	fmt.Printf("\n%d symbols found\n", len(symbols))

	return nil
}

func runFindRefs(cmd *cobra.Command, args []string) error {
	symbol := args[0]
	output := viper.GetString("output")

	repos, _ := cmd.Flags().GetStringSlice("repos")
	lang, _ := cmd.Flags().GetString("lang")
	limit, _ := cmd.Flags().GetInt("limit")

	c := newClient()

	refs, err := c.FindRefs(context.Background(), &client.FindRefsRequest{
		Symbol:   symbol,
		Repos:    repos,
		Language: lang,
		Limit:    limit,
	})
	if err != nil {
		return fmt.Errorf("find refs failed: %w", err)
	}

	switch output {
	case "json":
		return outputJSON(refs)
	default:
		return outputRefsText(refs)
	}
}

func outputRefsText(refs []client.Reference) error {
	if len(refs) == 0 {
		fmt.Println("No references found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "FILE\tLINE\tCONTEXT\tREPO\n")

	for _, r := range refs {
		content := truncate(r.Context, 60)
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", r.File, r.Line, content, r.Repo)
	}

	w.Flush()

	fmt.Printf("\n%d references found\n", len(refs))

	return nil
}
