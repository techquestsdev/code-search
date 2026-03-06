package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/techquestsdev/code-search/cmd/cli/client"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage repositories",
}

var repoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List indexed repositories",
	RunE:  runRepoList,
}

var repoAddCmd = &cobra.Command{
	Use:   "add <clone-url>",
	Short: "Add a repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoAdd,
}

var repoSyncCmd = &cobra.Command{
	Use:   "sync [id]",
	Short: "Sync repository by ID or all",
	RunE:  runRepoSync,
}

var repoStatusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Show repository status by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoStatus,
}

var repoRemoveCmd = &cobra.Command{
	Use:   "remove <id>",
	Short: "Remove a repository by ID",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoRemove,
}

var repoExcludeCmd = &cobra.Command{
	Use:   "exclude <id>",
	Short: "Exclude a repository from sync and indexing",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoExclude,
}

var repoIncludeCmd = &cobra.Command{
	Use:   "include <id>",
	Short: "Include a previously excluded repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoInclude,
}

var repoRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a previously deleted repository",
	Args:  cobra.ExactArgs(1),
	RunE:  runRepoRestore,
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoAddCmd)
	repoCmd.AddCommand(repoSyncCmd)
	repoCmd.AddCommand(repoStatusCmd)
	repoCmd.AddCommand(repoRemoveCmd)
	repoCmd.AddCommand(repoExcludeCmd)
	repoCmd.AddCommand(repoIncludeCmd)
	repoCmd.AddCommand(repoRestoreCmd)

	repoListCmd.Flags().Int64P("connection", "c", 0, "Filter by connection ID")
	repoListCmd.Flags().BoolP("verbose", "v", false, "Show detailed info")

	repoAddCmd.Flags().Int64("connection", 0, "Connection ID (required)")
	repoAddCmd.Flags().String("name", "", "Repository name")
	repoAddCmd.Flags().String("branch", "main", "Default branch")
	repoAddCmd.MarkFlagRequired("connection")

	repoSyncCmd.Flags().Bool("all", false, "Sync all repositories")

	repoRemoveCmd.Flags().Bool("force", false, "Skip confirmation")
}

func runRepoList(cmd *cobra.Command, args []string) error {
	connectionID, _ := cmd.Flags().GetInt64("connection")
	verbose, _ := cmd.Flags().GetBool("verbose")

	c := newClient()

	var connIDPtr *int64
	if connectionID > 0 {
		connIDPtr = &connectionID
	}

	resp, err := c.ListRepositories(context.Background(), connIDPtr)
	if err != nil {
		return fmt.Errorf("list repositories: %w", err)
	}

	if len(resp.Repos) == 0 {
		fmt.Println("No repositories found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if verbose {
		fmt.Fprintf(w, "ID\tNAME\tSTATUS\tLAST INDEXED\tEXCLUDED\tDELETED\tURL\n")

		for _, r := range resp.Repos {
			lastIndexed := r.LastIndexed
			if lastIndexed == "" {
				lastIndexed = "-"
			}

			excluded := ""
			if r.Excluded {
				excluded = "yes"
			}

			deleted := ""
			if r.Deleted {
				deleted = "yes"
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.ID, r.Name, statusIcon(r.Status), lastIndexed, excluded, deleted, r.CloneURL)
		}
	} else {
		fmt.Fprintf(w, "ID\tNAME\tSTATUS\tBRANCHES\n")

		for _, r := range resp.Repos {
			branches := "-"
			if len(r.Branches) > 0 {
				branches = strings.Join(r.Branches, ", ")
			}

			fmt.Fprintf(w, "%d\t%s\t%s\t%s\n", r.ID, r.Name, statusIcon(r.Status), branches)
		}
	}

	w.Flush()

	fmt.Printf("\n%d repositories\n", resp.TotalCount)

	return nil
}

func statusIcon(status string) string {
	switch status {
	case "indexed":
		return "✓ indexed"
	case "pending":
		return "⏳ pending"
	case "cloning":
		return "📥 cloning"
	case "indexing":
		return "🔄 indexing"
	case "failed":
		return "✗ failed"
	case "excluded":
		return "— excluded"
	case "deleted":
		return "🗑 deleted"
	default:
		return status
	}
}

func runRepoAdd(cmd *cobra.Command, args []string) error {
	cloneURL := args[0]

	connectionID, _ := cmd.Flags().GetInt64("connection")
	name, _ := cmd.Flags().GetString("name")
	branch, _ := cmd.Flags().GetString("branch")

	c := newClient()

	repo, err := c.AddRepository(context.Background(), &client.AddRepositoryRequest{
		ConnectionID:  connectionID,
		Name:          name,
		CloneURL:      cloneURL,
		DefaultBranch: branch,
	})
	if err != nil {
		return fmt.Errorf("add repository: %w", err)
	}

	fmt.Printf("✓ Added repository: %s (ID: %d)\n", repo.Name, repo.ID)

	return nil
}

func runRepoSync(cmd *cobra.Command, args []string) error {
	syncAll, _ := cmd.Flags().GetBool("all")

	c := newClient()

	if syncAll {
		err := c.SyncAllRepositories(context.Background())
		if err != nil {
			return fmt.Errorf("sync all: %w", err)
		}

		fmt.Println("✓ Sync triggered for all repositories")
	} else if len(args) > 0 {
		repoID, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid repository ID: %w", err)
		}

		if err := c.SyncRepository(context.Background(), repoID); err != nil {
			return fmt.Errorf("sync repository: %w", err)
		}

		fmt.Printf("✓ Sync triggered for repository ID: %d\n", repoID)
	} else {
		return errors.New("please specify a repository ID or use --all")
	}

	return nil
}

func runRepoStatus(cmd *cobra.Command, args []string) error {
	repoID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	c := newClient()

	repo, err := c.GetRepository(context.Background(), repoID)
	if err != nil {
		return fmt.Errorf("get repository: %w", err)
	}

	fmt.Printf("ID:         %d\n", repo.ID)
	fmt.Printf("Name:       %s\n", repo.Name)
	fmt.Printf("Status:     %s\n", statusIcon(repo.Status))
	fmt.Printf("URL:        %s\n", repo.CloneURL)

	if len(repo.Branches) > 0 {
		fmt.Printf("Branches:   %s\n", strings.Join(repo.Branches, ", "))
	}

	if repo.LastIndexed != "" {
		fmt.Printf("Indexed:    %s\n", repo.LastIndexed)
	}

	if repo.Excluded {
		fmt.Printf("Excluded:   yes\n")
	}

	if repo.Deleted {
		fmt.Printf("Deleted:    yes\n")
	}

	return nil
}

func runRepoRemove(cmd *cobra.Command, args []string) error {
	repoID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	force, _ := cmd.Flags().GetBool("force")

	if !force {
		fmt.Printf("Remove repository ID '%d'? This will soft-delete the repository (can be restored later). [y/N]: ", repoID)

		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Aborted")
			return nil
		}
	}

	c := newClient()
	if err := c.RemoveRepository(context.Background(), repoID); err != nil {
		return fmt.Errorf("remove repository: %w", err)
	}

	fmt.Printf("✓ Removed repository ID: %d\n", repoID)

	return nil
}

func runRepoExclude(cmd *cobra.Command, args []string) error {
	repoID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	c := newClient()
	if err := c.ExcludeRepository(context.Background(), repoID); err != nil {
		return fmt.Errorf("exclude repository: %w", err)
	}

	fmt.Printf("✓ Excluded repository ID: %d\n", repoID)

	return nil
}

func runRepoInclude(cmd *cobra.Command, args []string) error {
	repoID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	c := newClient()
	if err := c.IncludeRepository(context.Background(), repoID); err != nil {
		return fmt.Errorf("include repository: %w", err)
	}

	fmt.Printf("✓ Included repository ID: %d\n", repoID)

	return nil
}

func runRepoRestore(cmd *cobra.Command, args []string) error {
	repoID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid repository ID: %w", err)
	}

	c := newClient()
	if err := c.RestoreRepository(context.Background(), repoID); err != nil {
		return fmt.Errorf("restore repository: %w", err)
	}

	fmt.Printf("✓ Restored repository ID: %d\n", repoID)

	return nil
}
