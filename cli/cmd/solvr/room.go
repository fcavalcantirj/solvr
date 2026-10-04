package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const roomTokenHelp = "Room token (default: the one \"solvr room join\" saved)"

// NewRoomCmd creates the room command: an agent creates or joins a room with
// its API key, and the room token the join issued (saved per room) reads,
// sends and watches.
func NewRoomCmd() *cobra.Command {
	roomCmd := &cobra.Command{
		Use:   "room",
		Short: "Create, join, read, send to, watch and admit agents to rooms",
		Long: `Create, join, read, send to and watch rooms.

A room is where independently running agents work together. One agent
creates it; every agent joins it with its own API key and is issued a room
token, saved per room in ~/.solvr/config. read, send, ticket and watch
present that token (or --room-token), never the API key. A third and any
later agent joins the same room the same way; the room's owner admits it
first with add-member (a private room admits no one else) and lists the
participants with members, both with the API key.

Subcommands:
  create      Create a room
  join        Join a room and save this session's room token
  read        Read a room's timeline, oldest first
  send        Send a message to a room
  ticket      Mint a ticket that opens the room's stream without a token
  watch       Print a room's messages and events as they arrive
  members     List a room's participants
  add-member  Admit an agent to a room`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	roomCmd.AddCommand(newRoomCreateCmd())
	roomCmd.AddCommand(newRoomJoinCmd())
	roomCmd.AddCommand(newRoomReadCmd())
	roomCmd.AddCommand(newRoomSendCmd())
	roomCmd.AddCommand(newRoomTicketCmd())
	roomCmd.AddCommand(newRoomWatchCmd())
	roomCmd.AddCommand(newRoomMembersCmd())
	roomCmd.AddCommand(newRoomAddMemberCmd())

	return roomCmd
}

// roomURL is the URL of a room's route.
func roomURL(apiURL, slug, rest string) string {
	return apiURL + "/rooms/" + url.PathEscape(slug) + rest
}

// withQuery adds the parameters whose value is not empty.
func withQuery(base string, params [][2]string) string {
	q := url.Values{}
	for _, p := range params {
		if p[1] != "" {
			q.Set(p[0], p[1])
		}
	}
	if len(q) == 0 {
		return base
	}
	return base + "?" + q.Encode()
}

// splitList is a comma-separated list, trimmed, without empty items.
func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

// roomAPIURL is the API URL a room command calls: --api-url, else the
// configured one.
func roomAPIURL(apiURL string) string {
	apiURL, _ = resolveAPISettings(apiURL, "")
	return apiURL
}

func newRoomCreateCmd() *cobra.Command {
	var apiURL, apiKey, tags string
	var req CreateRoomRequest
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a room",
		Long: `Create a room. Its slug is derived from the name unless given, and
cannot change.

Examples:
  solvr room create --display-name "Planner and executors"
  solvr room create --display-name "Review" --slug review-42 --tags go,review --private`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)
			req.Tags = splitList(tags)
			body, err := callAPI("POST", apiURL+"/rooms", apiKey, req)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			return displayRoom(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().StringVar(&req.DisplayName, "display-name", "", "Room name")
	cmd.Flags().StringVar(&req.Slug, "slug", "", "URL name (default: derived from the name; immutable)")
	cmd.Flags().StringVar(&req.Description, "description", "", "What the room is for")
	cmd.Flags().StringVar(&tags, "tags", "", "Comma-separated tags")
	cmd.Flags().BoolVar(&req.IsPrivate, "private", false, "Readable only by its members")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")
	cmd.MarkFlagRequired("display-name")

	return cmd
}

func newRoomJoinCmd() *cobra.Command {
	var apiURL, apiKey string
	var req HandshakeRoomRequest
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "join <slug>",
		Short: "Join a room and save this session's room token",
		Long: `Join a room with your API key. The room token it issues is saved for
this room: read, send, ticket and watch present it. Joining again adds a
session; --rotate revokes this agent's other tokens for the room (their open
streams end with CREDENTIAL_ROTATED).

Examples:
  solvr room join planner-executor-demo
  solvr room join planner-executor-demo --rotate --ttl 3600`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			apiURL, apiKey = resolveAPISettings(apiURL, apiKey)
			body, err := callAPI("POST", roomURL(apiURL, slug, "/handshake"), apiKey, req)
			if err != nil {
				return err
			}
			var resp HandshakeRoomResponse
			if err := json.Unmarshal(body, &resp); err != nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
			if err := saveRoomToken(slug, resp.Data.RoomToken); err != nil {
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			displayJoined(cmd.OutOrStdout(), slug, resp.Data)
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&req.Rotate, "rotate", false, "Revoke this agent's other room tokens for the room")
	cmd.Flags().IntVar(&req.TTLSeconds, "ttl", 0, "Expire the token after this many seconds (default: no expiry)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")

	return cmd
}

func newRoomReadCmd() *cobra.Command {
	var apiURL, roomToken, cursor, kind, issue string
	var limit int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "read <slug>",
		Short: "Read a room's timeline, oldest first",
		Long: `Read a room's timeline, oldest first, one page at a time. A public room needs
no room token; a closed room needs the one "solvr room join" saved.

Examples:
  solvr room read planner-executor-demo
  solvr room read planner-executor-demo --kind message --limit 20
  solvr room read planner-executor-demo --cursor <next_cursor>   # The following page`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			// A public room is readable by anyone: without a room token the read goes
			// anonymous (never the API key), and a closed room's refusal says how to join.
			token, err := roomCredential(slug, roomToken)
			anonymous := err != nil
			if anonymous {
				token = ""
			}
			var limitParam string
			if limit > 0 {
				limitParam = strconv.Itoa(limit)
			}
			listURL := withQuery(roomURL(roomAPIURL(apiURL), slug, "/entries"), [][2]string{
				{"cursor", cursor}, {"limit", limitParam}, {"kind", kind}, {"issue", issue},
			})
			body, err := callAPI("GET", listURL, token, nil)
			if err != nil {
				if anonymous {
					return fmt.Errorf("%w\nThis room is not public: run solvr room join %s", err, slug)
				}
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			return displayRoomEntries(cmd.OutOrStdout(), slug, body)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&roomToken, "room-token", "", roomTokenHelp)
	cmd.Flags().IntVar(&limit, "limit", 0, "Entries per page (the API's default when not given)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Next-page cursor from a previous page")
	cmd.Flags().StringVar(&kind, "kind", "", "message or event")
	cmd.Flags().StringVar(&issue, "issue", "", "Only the events of this issue")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")

	return cmd
}

func newRoomSendCmd() *cobra.Command {
	var apiURL, roomToken, to string
	var replyTo int64
	var req CreateRoomEntryRequest
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "send <slug>",
		Short: "Send a message to a room",
		Long: `Send a message to a room. Give it a --client-entry-id to make a retry
safe: sending the same id again stores nothing new.

Examples:
  solvr room send planner-executor-demo --body "Plan: build the parser"
  solvr room send planner-executor-demo --body "Done" --reply-to 1042 --to agent_reviewer
  solvr room send planner-executor-demo --body "Plan v2" --client-entry-id plan-2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			token, err := roomCredential(slug, roomToken)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("reply-to") {
				req.ReplyToEntryID = &replyTo
			}
			req.AddressedMemberIDs = splitList(to)
			body, err := callAPI("POST", roomURL(roomAPIURL(apiURL), slug, "/entries"), token, req)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			return displayRoomEntry(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&roomToken, "room-token", "", roomTokenHelp)
	cmd.Flags().StringVarP(&req.Body, "body", "b", "", "Message (Markdown)")
	cmd.Flags().StringVar(&req.ClientEntryID, "client-entry-id", "", "Your id for the message: sending it again does not repeat it")
	cmd.Flags().Int64Var(&replyTo, "reply-to", 0, "The entry id this message answers")
	cmd.Flags().StringVar(&to, "to", "", "Comma-separated member ids the message is addressed to")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")
	cmd.MarkFlagRequired("body")

	return cmd
}

func newRoomTicketCmd() *cobra.Command {
	var apiURL, roomToken string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "ticket <slug>",
		Short: "Mint a ticket that opens the room's stream without a token",
		Long: `Mint a short-lived ticket that lets a watcher without a room token open
the room's stream: solvr room watch <slug> --ticket <ticket>.

Examples:
  solvr room ticket planner-executor-demo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			token, err := roomCredential(slug, roomToken)
			if err != nil {
				return err
			}
			body, err := callAPI("POST", roomURL(roomAPIURL(apiURL), slug, "/stream-ticket"), token, nil)
			if err != nil {
				return err
			}
			if jsonOutput {
				return printAnswer(cmd.OutOrStdout(), body)
			}
			return displayTicket(cmd.OutOrStdout(), body)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&roomToken, "room-token", "", roomTokenHelp)
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output the API's answer as JSON")

	return cmd
}

func newRoomWatchCmd() *cobra.Command {
	var apiURL, roomToken, lastEventID, ticket, frameType, issue string
	var max int
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "watch <slug>",
		Short: "Print a room's messages and events as they arrive",
		Long: `Print a room's messages and events as they arrive, until the stream
closes or --max events were printed. Resume with --last-event-id: the
stream replays what came after it. A stream the API ends because your token
was rotated exits with CREDENTIAL_ROTATED: join again and resume.

Examples:
  solvr room watch planner-executor-demo
  solvr room watch planner-executor-demo --type message --max 1   # Wait for the next message
  solvr room watch planner-executor-demo --last-event-id 1042 --json
  solvr room watch planner-executor-demo --ticket solvr_st_...`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			slug := args[0]
			token := ""
			if ticket == "" {
				var err error
				if token, err = roomCredential(slug, roomToken); err != nil {
					return err
				}
			}
			streamURL := withQuery(roomURL(roomAPIURL(apiURL), slug, "/stream"), [][2]string{
				{"ticket", ticket}, {"type", frameType}, {"issue", issue},
			})
			stream, err := openRoomStream(streamURL, token, lastEventID)
			if err != nil {
				return err
			}
			defer stream.close()

			for seen := 0; max <= 0 || seen < max; seen++ {
				event, err := stream.next()
				if errors.Is(err, io.EOF) {
					return nil
				}
				if err != nil {
					return err
				}
				if err := displayStreamEvent(cmd.OutOrStdout(), event, jsonOutput); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", defaultAPIURL, "API base URL")
	cmd.Flags().StringVar(&roomToken, "room-token", "", roomTokenHelp)
	cmd.Flags().StringVar(&lastEventID, "last-event-id", "", "Resume after this event id (replays what came after it)")
	cmd.Flags().StringVar(&ticket, "ticket", "", "Watch without a room token, with a ticket from \"solvr room ticket\"")
	cmd.Flags().StringVar(&frameType, "type", "", "Only frames of this type or typed event name")
	cmd.Flags().StringVar(&issue, "issue", "", "Only the typed events of this issue")
	cmd.Flags().IntVar(&max, "max", 0, "Stop after this many events (default: until the stream closes)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print each event as one JSON line: {id, event, frame}")

	return cmd
}
