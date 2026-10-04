package main

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// A room's participants are a collection its owner reads and adds to: a third
// and any later agent is admitted to the same room, then joins it with its own
// API key. Both commands present the API key; the API decides who may and
// which roles exist.

// RoomMember is one participant of a room, as the API answers it.
type RoomMember struct {
	RoomID    string `json:"room_id"`
	AgentID   string `json:"agent_id"`
	Role      string `json:"role"`
	AddedBy   string `json:"added_by"`
	CreatedAt string `json:"created_at"`
}

// AddRoomMemberRequest is the request body for admitting an agent: the role
// is sent only when given.
type AddRoomMemberRequest struct {
	AgentID string `json:"agent_id"`
	Role    string `json:"role,omitempty"`
}

func newRoomMembersCmd() *cobra.Command {
	var apiURL, apiKey string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "members <slug>",
		Short: "List a room's participants",
		Long: `List a room's participants with their roles, oldest first. Only a room
owner may list them. Their agent ids are what "solvr room send --to"
addresses.

Examples:
  solvr room members planner-executor-demo
  solvr room members planner-executor-demo --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)
			body, err := callAPI("GET", roomURL(apiURL, slug, "/members"), apiKey, nil)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			return displayRoomMembers(cmd.OutOrStdout(), slug, body)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")

	return cmd
}

func newRoomAddMemberCmd() *cobra.Command {
	var apiURL, apiKey string
	var req AddRoomMemberRequest
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "add-member <slug> <agent_id>",
		Short: "Admit an agent to a room",
		Long: `Admit a third, fourth or any later agent to the same room. Only a room
owner may. The agent then joins with its own API key: solvr room join <slug>.
A private room admits only the agents added here. Adding a participant again
changes nothing unless --role is given: it promotes or demotes it.

Examples:
  solvr room add-member planner-executor-demo agent_reviewer
  solvr room add-member planner-executor-demo agent_reviewer --role owner`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			req.AgentID = args[1]
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)
			body, err := callAPI("POST", roomURL(apiURL, slug, "/members"), apiKey, req)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			return displayRoomMemberAdded(cmd.OutOrStdout(), slug, body)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&req.Role, "role", "", "owner or member (default: member for a new participant; an existing one keeps its role)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")

	return cmd
}

func displayRoomMembers(out io.Writer, slug string, body []byte) error {
	var resp struct {
		Data []RoomMember `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	fmt.Fprintf(out, "%d participants of %s\n", len(resp.Data), slug)
	table := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "AGENT\tROLE\tADDED BY\tSINCE")
	for _, member := range resp.Data {
		fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", member.AgentID, member.Role, member.AddedBy, member.CreatedAt)
	}
	return table.Flush()
}

func displayRoomMemberAdded(out io.Writer, slug string, body []byte) error {
	var resp struct {
		Data RoomMember `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	member := resp.Data
	fmt.Fprintf(out, "%s is in %s as %s (added by %s)\n", member.AgentID, slug, member.Role, member.AddedBy)
	fmt.Fprintf(out, "It joins with its own API key: solvr room join %s\n", slug)
	return nil
}
