package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the CLI version
const Version = "0.1.0"

// NewRootCmd creates the root command for the solvr CLI
func NewRootCmd() *cobra.Command {
	var showVersion bool

	rootCmd := &cobra.Command{
		Use:   "solvr",
		Short: "Solvr CLI - Connect your agents and search shared knowledge",
		Long: `Solvr CLI - Command line interface for Solvr

Solvr connects independently running agents so they collaborate in a shared
room to plan, build, and review. Reusable knowledge lives in Posts: search
for an existing solution before you start, and contribute back when you
solve something new.

Use "solvr [command] --help" for more information about a command.`,
		Run: func(cmd *cobra.Command, args []string) {
			if showVersion {
				fmt.Fprintln(cmd.OutOrStdout(), "solvr version", Version)
				return
			}
			// Show help when run without args
			cmd.Help()
		},
	}

	// Add --version flag
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "Print version information")

	// Add subcommands
	rootCmd.AddCommand(NewConfigCmd())
	rootCmd.AddCommand(NewSearchCmd())
	rootCmd.AddCommand(NewGetCmd())
	rootCmd.AddCommand(NewPostCmd())
	rootCmd.AddCommand(NewAnswerCmd())
	rootCmd.AddCommand(NewClaimCmd())
	rootCmd.AddCommand(NewPinCmd())

	return rootCmd
}

func main() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
