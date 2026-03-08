package cmd

import (
	"fmt"
	"os"

	"github.com/nicholascross/opengit/internal/app"
	"github.com/nicholascross/opengit/internal/version"
	"github.com/spf13/cobra"
)

var (
	repoPath string
	debug    bool
)

var rootCmd = &cobra.Command{
	Use:   "opengit",
	Short: "Terminal-native git client",
	Long:  "opengit is a terminal-native git client that replaces GitKraken, SourceTree, and GitHub Desktop.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return app.Run(repoPath, debug)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("opengit %s (%s) built %s\n", version.Version, version.Commit, version.Date)
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&repoPath, "repo", "r", ".", "Path to git repository")
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging")
	rootCmd.AddCommand(versionCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
