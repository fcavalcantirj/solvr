package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// 0.2.0 removed the 0.1 choices of the legacy knowledge model. They still parse,
// so a 0.1 invocation fails before any request, naming what replaces it, instead
// of failing as an unknown command or flag or running without the choice.

const migratingHelp = `Migrating from 0.1 to ` + Version + `

0.2.0 removes the choices of the legacy knowledge model: a post has no type,
every contribution to a post is a reply, a post's replies are read on their
own, and search covers every post. A removed command, argument or flag is
refused before any request, with exit code 1 and what replaces it.

  0.1                                     0.2.0
  solvr post <type> --title --description solvr post --title "..." --description "..."
    (problem, question or idea)             (a post has no type)
  solvr answer <post_id> --content        solvr reply <post_id> --body "..."
    (or --editor)                           (or --editor)
  solvr get <id> --include                solvr get <id>, then solvr replies <id>: the
    approaches,answers,responses            answers and approaches from before are replies
  solvr search <query> --type             solvr search <query> searches every post

Also in 0.2.0: solvr get-reply and solvr update-reply read and edit a reply,
and solvr room creates, joins, reads, sends to and watches a room.

A 0.1 CLI that is still installed fails on answer and get --include: the API
retired the routes they call and answers them 410 ENDPOINT_RETIRED, naming the
route to use in error.details.replacement.`

// NewMigratingCmd is the help topic of the removed choices: solvr help migrating.
func NewMigratingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrating",
		Short: "Migrating from 0.1 to " + Version + ": the removed commands and flags",
		Long:  migratingHelp,
	}
}

// removedError is the error of a removed 0.1 command, argument or flag.
func removedError(name, instead string) error {
	return fmt.Errorf("'%s' was removed in solvr %s; %s. Run 'solvr help migrating'", name, Version, instead)
}

// removedCommand keeps a removed command out of --help; running it, with any
// arguments or flags, fails.
func removedCommand(name, instead string) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Hidden:             true,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removedError("solvr "+name, instead)
		},
	}
}

// removeFlag keeps a removed flag of cmd out of --help; giving it fails before
// the command runs.
func removeFlag(cmd *cobra.Command, name, instead string) {
	cmd.Flags().String(name, "", "")
	_ = cmd.Flags().MarkHidden(name)
	next := cmd.PreRunE
	cmd.PreRunE = func(c *cobra.Command, args []string) error {
		if c.Flags().Changed(name) {
			c.SilenceUsage = true
			return removedError("--"+name, instead)
		}
		if next != nil {
			return next(c, args)
		}
		return nil
	}
}
