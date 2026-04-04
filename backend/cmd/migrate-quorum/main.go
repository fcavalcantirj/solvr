// Package main implements the migrate-quorum CLI tool.
// It migrates 5 rooms and their messages from Quorum's PostgreSQL database
// into Solvr's PostgreSQL database using a single atomic transaction.
//
// Usage:
//
//	QUORUM_DB_URL="postgres://..." DATABASE_URL="postgres://..." go run ./cmd/migrate-quorum/ [--dry-run]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Per D-13: ONLY these 5 rooms are migrated.
var allowedSlugs = map[string]bool{
	"ballona-trade-v0":      true,
	"composio-integration":  true,
	"mackjack-ops":          true,
	"jack-mack-msv-trading": true,
	"solvr-usage-analysys":  true, // original Quorum slug (typo preserved)
}

// Per D-35: slug corrections applied at target.
var slugTransforms = map[string]string{
	"solvr-usage-analysys": "solvr-usage-analysis",
}

// Per D-18: hardcoded room slug -> owner email mapping.
var slugOwnerEmail = map[string]string{
	"composio-integration":  "felipecavalcantirj@gmail.com",
	"solvr-usage-analysys":  "felipecavalcantirj@gmail.com",
	"ballona-trade-v0":      "macballona@gmail.com",
	"mackjack-ops":          "macballona@gmail.com",
	"jack-mack-msv-trading": "macballona@gmail.com",
}

// Per D-21: pre-existing Solvr agent IDs for known Quorum agents.
var knownAgentIDs = map[string]string{
	"ClaudiusThePirateEmperor": "agent_ClaudiusThePirateEmperor",
	"Jack":                     "agent_Jack",
	"Mack":                     "agent_Mack",
}

// Per D-25: markdown detection patterns.
// Matches: headers (#), bold (**text**), code blocks (```), inline code (`code`).
var markdownPattern = regexp.MustCompile("(?m)(^#{1,6} |\\*\\*[^*]+\\*\\*|^```|`[^`]+`)")

// quorumRoom represents a room read from Quorum DB.
type quorumRoom struct {
	ID          uuid.UUID
	Slug        string
	DisplayName string
	TokenHash   string
	OwnerID     *uuid.UUID // nullable in Quorum
	CreatedAt   time.Time
}

// quorumMessage represents a message read from Quorum DB.
type quorumMessage struct {
	AgentName string
	Content   string
	CreatedAt time.Time
}

// roomInsert represents a room to insert into Solvr.
type roomInsert struct {
	ID          uuid.UUID
	Slug        string
	DisplayName string
	IsPrivate   bool
	OwnerID     uuid.UUID
	TokenHash   string
	CreatedAt   time.Time
}

// agentInsert represents an agent to register in Solvr.
type agentInsert struct {
	ID          string // "agent_{name}", max 50 chars
	DisplayName string
	HumanID     uuid.UUID // owner's Solvr user UUID
}

// messageInsert represents a message to insert into Solvr.
type messageInsert struct {
	RoomID      uuid.UUID
	AuthorType  string // always "agent"
	AuthorID    string // agent ID in Solvr
	AgentName   string
	Content     string
	ContentType string // "text" or "markdown"
	Metadata    string // always "{}"
	SequenceNum int
	CreatedAt   time.Time
}

// migrationResult holds the summary counts from a migration run.
type migrationResult struct {
	RoomsMigrated    int
	RoomsSkipped     int
	MessagesMigrated int
	AgentsCreated    int
	AgentsExisting   int
}

// txInterface abstracts pgx transaction methods for testability.
// This is a subset of db.Tx that our migration code actually uses.
type txInterface interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// migrationDB abstracts all database operations for testability.
type migrationDB interface {
	// Quorum reads
	ListQuorumRooms(ctx context.Context) ([]quorumRoom, error)
	ListQuorumMessages(ctx context.Context, roomID uuid.UUID) ([]quorumMessage, error)
	// Solvr reads
	FindSolvrUserByEmail(ctx context.Context, email string) (uuid.UUID, error)
	CheckSlugConflict(ctx context.Context, slug string, roomID uuid.UUID) (bool, error)
	// Solvr writes (through transaction)
	BeginTx(ctx context.Context) (txInterface, error)
	InsertRoom(ctx context.Context, tx txInterface, r roomInsert) (bool, error)
	InsertAgent(ctx context.Context, tx txInterface, a agentInsert) (bool, error)
	InsertMessagesWithSequence(ctx context.Context, tx txInterface, roomID uuid.UUID, msgs []messageInsert) (int, error)
	UpdateRoomMessageCount(ctx context.Context, tx txInterface, roomID uuid.UUID) error
}

// migrator orchestrates the migration from Quorum to Solvr.
type migrator struct {
	db     migrationDB
	dryRun bool
}

// detectContentType returns "markdown" if the content contains markdown patterns,
// otherwise "text". Per D-25.
func detectContentType(content string) string {
	if markdownPattern.MatchString(content) {
		return "markdown"
	}
	return "text"
}

// targetSlug applies slug transformations (e.g., typo corrections). Per D-35.
func targetSlug(quorumSlug string) string {
	if corrected, ok := slugTransforms[quorumSlug]; ok {
		return corrected
	}
	return quorumSlug
}

// agentID returns the Solvr agent ID for a given Quorum agent name.
// Known agents use their pre-existing IDs; others get "agent_{name}" truncated to 50 chars.
// Per D-21, D-22.
func agentID(agentName string) string {
	if id, ok := knownAgentIDs[agentName]; ok {
		return id
	}
	id := "agent_" + agentName
	if len(id) > 50 {
		id = id[:50]
	}
	return id
}

// cleanDisplayName removes/replaces special characters in display names. Per D-34.
func cleanDisplayName(name string) string {
	name = strings.ReplaceAll(name, "&", "and")
	name = strings.TrimSpace(name)
	return name
}

// run executes the full migration and returns a summary.
func (m *migrator) run(ctx context.Context) (*migrationResult, error) {
	result := &migrationResult{}

	// Step 1: Resolve owner UUIDs — fail fast if not found (Pitfall 1).
	ownerUUIDs := make(map[string]uuid.UUID) // email -> solvr UUID
	emails := map[string]bool{}
	for _, email := range slugOwnerEmail {
		emails[email] = true
	}
	for email := range emails {
		uid, err := m.db.FindSolvrUserByEmail(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("resolve owner %s: %w", email, err)
		}
		ownerUUIDs[email] = uid
		slog.Info("resolved owner", "email", email, "uuid", uid)
	}

	// Step 2: List all Quorum rooms, filter to allowedSlugs.
	allRooms, err := m.db.ListQuorumRooms(ctx)
	if err != nil {
		return nil, fmt.Errorf("list quorum rooms: %w", err)
	}

	type roomPlan struct {
		quorum  quorumRoom
		insert  roomInsert
		msgs    []quorumMessage
		ownerID uuid.UUID
	}
	var plans []roomPlan

	for _, qr := range allRooms {
		if !allowedSlugs[qr.Slug] {
			result.RoomsSkipped++
			slog.Debug("skipping room not in allow-list", "slug", qr.Slug)
			continue
		}

		tSlug := targetSlug(qr.Slug)
		ownerEmail := slugOwnerEmail[qr.Slug]
		ownerID := ownerUUIDs[ownerEmail]

		// Check for slug conflict (D-53).
		conflict, err := m.db.CheckSlugConflict(ctx, tSlug, qr.ID)
		if err != nil {
			return nil, fmt.Errorf("check slug conflict for %s: %w", tSlug, err)
		}
		if conflict {
			slog.Warn("slug conflict detected, skipping room",
				"quorum_slug", qr.Slug,
				"target_slug", tSlug,
				"room_id", qr.ID,
			)
			result.RoomsSkipped++
			continue
		}

		// Fetch messages for this room.
		msgs, err := m.db.ListQuorumMessages(ctx, qr.ID)
		if err != nil {
			return nil, fmt.Errorf("list messages for room %s: %w", qr.Slug, err)
		}

		ri := roomInsert{
			ID:          qr.ID,
			Slug:        tSlug,
			DisplayName: cleanDisplayName(qr.DisplayName),
			IsPrivate:   false,
			OwnerID:     ownerID,
			TokenHash:   qr.TokenHash,
			CreatedAt:   qr.CreatedAt,
		}
		plans = append(plans, roomPlan{
			quorum:  qr,
			insert:  ri,
			msgs:    msgs,
			ownerID: ownerID,
		})
	}

	// Step 3: Dry-run — log summary and return without writing.
	if m.dryRun {
		totalMsgs := 0
		agentNames := make(map[string]bool)
		for _, p := range plans {
			totalMsgs += len(p.msgs)
			for _, msg := range p.msgs {
				agentNames[msg.AgentName] = true
			}
		}
		slog.Info("dry-run summary",
			"rooms_to_migrate", len(plans),
			"rooms_to_skip", result.RoomsSkipped,
			"messages_to_migrate", totalMsgs,
			"unique_agents", len(agentNames),
		)
		fmt.Printf("Dry run: would migrate %d rooms, %d messages, %d agents\n",
			len(plans), totalMsgs, len(agentNames))
		result.RoomsMigrated = len(plans)
		result.MessagesMigrated = totalMsgs
		result.AgentsCreated = len(agentNames)
		return result, nil
	}

	// Step 4: Begin single transaction on Solvr DB (D-05).
	tx, err := m.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}

	// Ensure rollback on any error.
	committed := false
	defer func() {
		if !committed {
			if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				slog.Error("rollback failed", "error", rbErr)
			}
		}
	}()

	// Step 5: Process each room.
	for _, p := range plans {
		// Insert room (ON CONFLICT DO NOTHING).
		inserted, err := m.db.InsertRoom(ctx, tx, p.insert)
		if err != nil {
			return nil, fmt.Errorf("insert room %s: %w", p.insert.Slug, err)
		}
		if inserted {
			result.RoomsMigrated++
		}

		// Collect unique agent names from this room's messages.
		agentNamesSeen := make(map[string]bool)
		for _, msg := range p.msgs {
			agentNamesSeen[msg.AgentName] = true
		}

		// Insert agents (ON CONFLICT DO NOTHING).
		for name := range agentNamesSeen {
			ai := agentInsert{
				ID:          agentID(name),
				DisplayName: name,
				HumanID:     p.ownerID,
			}
			agentInserted, err := m.db.InsertAgent(ctx, tx, ai)
			if err != nil {
				return nil, fmt.Errorf("insert agent %s: %w", ai.ID, err)
			}
			if agentInserted {
				result.AgentsCreated++
			} else {
				result.AgentsExisting++
			}
		}

		// Build message inserts with sequence numbers (1..N ordered by created_at).
		// Messages from ListQuorumMessages are already ordered by created_at.
		msgInserts := make([]messageInsert, 0, len(p.msgs))
		for i, qm := range p.msgs {
			mi := messageInsert{
				RoomID:      p.quorum.ID,
				AuthorType:  "agent",
				AuthorID:    agentID(qm.AgentName),
				AgentName:   qm.AgentName,
				Content:     qm.Content,
				ContentType: detectContentType(qm.Content),
				Metadata:    "{}",
				SequenceNum: i + 1,
				CreatedAt:   qm.CreatedAt,
			}
			msgInserts = append(msgInserts, mi)
		}

		if len(msgInserts) > 0 {
			count, err := m.db.InsertMessagesWithSequence(ctx, tx, p.quorum.ID, msgInserts)
			if err != nil {
				return nil, fmt.Errorf("insert messages for room %s: %w", p.insert.Slug, err)
			}
			result.MessagesMigrated += count
		}

		// Update room message_count (D-06).
		if err := m.db.UpdateRoomMessageCount(ctx, tx, p.quorum.ID); err != nil {
			return nil, fmt.Errorf("update message count for room %s: %w", p.insert.Slug, err)
		}

		slog.Info("migrated room",
			"slug", p.insert.Slug,
			"messages", len(msgInserts),
			"agents", len(agentNamesSeen),
		)
	}

	// Step 6: Commit transaction.
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}
	committed = true

	slog.Info("migration complete",
		"rooms_migrated", result.RoomsMigrated,
		"rooms_skipped", result.RoomsSkipped,
		"messages_migrated", result.MessagesMigrated,
		"agents_created", result.AgentsCreated,
		"agents_existing", result.AgentsExisting,
	)

	return result, nil
}

// pgMigrationDB implements migrationDB using real PostgreSQL connections.
type pgMigrationDB struct {
	quorum *pgxpool.Pool
	solvr  *db.Pool
}

func (d *pgMigrationDB) ListQuorumRooms(ctx context.Context) ([]quorumRoom, error) {
	rows, err := d.quorum.Query(ctx,
		`SELECT id, slug, display_name, token_hash, owner_id, created_at
		 FROM rooms ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("query quorum rooms: %w", err)
	}
	defer rows.Close()

	var rooms []quorumRoom
	for rows.Next() {
		var r quorumRoom
		if err := rows.Scan(&r.ID, &r.Slug, &r.DisplayName, &r.TokenHash, &r.OwnerID, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan quorum room: %w", err)
		}
		rooms = append(rooms, r)
	}
	return rooms, rows.Err()
}

func (d *pgMigrationDB) ListQuorumMessages(ctx context.Context, roomID uuid.UUID) ([]quorumMessage, error) {
	rows, err := d.quorum.Query(ctx,
		`SELECT agent_name, content, created_at
		 FROM messages WHERE room_id = $1 ORDER BY created_at`,
		roomID)
	if err != nil {
		return nil, fmt.Errorf("query quorum messages: %w", err)
	}
	defer rows.Close()

	var msgs []quorumMessage
	for rows.Next() {
		var m quorumMessage
		if err := rows.Scan(&m.AgentName, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan quorum message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func (d *pgMigrationDB) FindSolvrUserByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	var id uuid.UUID
	err := d.solvr.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("find user by email %s: %w", email, err)
	}
	return id, nil
}

func (d *pgMigrationDB) CheckSlugConflict(ctx context.Context, slug string, roomID uuid.UUID) (bool, error) {
	var exists bool
	err := d.solvr.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rooms WHERE slug = $1 AND id != $2)`,
		slug, roomID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check slug conflict: %w", err)
	}
	return exists, nil
}

func (d *pgMigrationDB) BeginTx(ctx context.Context) (txInterface, error) {
	return d.solvr.BeginTx(ctx)
}

func (d *pgMigrationDB) InsertRoom(ctx context.Context, tx txInterface, r roomInsert) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO rooms (id, slug, display_name, description, category, tags, is_private,
		                   owner_id, token_hash, message_count, created_at, updated_at,
		                   last_active_at, expires_at, deleted_at)
		VALUES ($1, $2, $3, NULL, NULL, '{}', $4, $5, $6, 0, $7, $7, $7, NULL, NULL)
		ON CONFLICT (id) DO NOTHING`,
		r.ID, r.Slug, r.DisplayName, r.IsPrivate, r.OwnerID, r.TokenHash, r.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert room %s: %w", r.Slug, err)
	}
	return tag.RowsAffected() > 0, nil
}

func (d *pgMigrationDB) InsertAgent(ctx context.Context, tx txInterface, a agentInsert) (bool, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO agents (id, display_name, human_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO NOTHING`,
		a.ID, a.DisplayName, a.HumanID,
	)
	if err != nil {
		return false, fmt.Errorf("insert agent %s: %w", a.ID, err)
	}
	return tag.RowsAffected() > 0, nil
}

func (d *pgMigrationDB) InsertMessagesWithSequence(ctx context.Context, tx txInterface, roomID uuid.UUID, msgs []messageInsert) (int, error) {
	count := 0
	for _, msg := range msgs {
		_, err := tx.Exec(ctx, `
			INSERT INTO messages (room_id, author_type, author_id, agent_name, content,
			                      content_type, metadata, sequence_num, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9)`,
			msg.RoomID, msg.AuthorType, msg.AuthorID, msg.AgentName, msg.Content,
			msg.ContentType, msg.Metadata, msg.SequenceNum, msg.CreatedAt,
		)
		if err != nil {
			return count, fmt.Errorf("insert message seq=%d: %w", msg.SequenceNum, err)
		}
		count++
	}
	return count, nil
}

func (d *pgMigrationDB) UpdateRoomMessageCount(ctx context.Context, tx txInterface, roomID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE rooms
		SET message_count = (SELECT COUNT(*) FROM messages WHERE room_id = $1 AND deleted_at IS NULL)
		WHERE id = $1`,
		roomID,
	)
	return err
}

func main() {
	dryRun := flag.Bool("dry-run", false, "Preview what would be migrated without writing to Solvr DB")
	flag.Parse()

	// Read QUORUM_DB_URL (D-47: never stored in files, passed at runtime).
	quorumDBURL := os.Getenv("QUORUM_DB_URL")
	if quorumDBURL == "" {
		log.Fatal("QUORUM_DB_URL environment variable is required")
	}

	// Read DATABASE_URL for Solvr (D-48).
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	ctx := context.Background()

	// Connect to Quorum (read-only source).
	quorumPool, err := pgxpool.New(ctx, quorumDBURL)
	if err != nil {
		log.Fatalf("Failed to connect to Quorum DB: %v", err)
	}
	defer quorumPool.Close()

	if err := quorumPool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping Quorum DB: %v", err)
	}
	slog.Info("connected to Quorum DB")

	// Connect to Solvr (transactional target).
	connectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	solvrPool, err := db.NewPool(connectCtx, databaseURL)
	cancel()
	if err != nil {
		log.Fatalf("Failed to connect to Solvr DB: %v", err)
	}
	defer solvrPool.Close()
	slog.Info("connected to Solvr DB")

	migDB := &pgMigrationDB{
		quorum: quorumPool,
		solvr:  solvrPool,
	}

	m := &migrator{
		db:     migDB,
		dryRun: *dryRun,
	}

	result, err := m.run(ctx)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Printf("\nMigration %s\n", map[bool]string{true: "DRY RUN complete", false: "complete"}[*dryRun])
	fmt.Printf("  Rooms migrated:    %d\n", result.RoomsMigrated)
	fmt.Printf("  Rooms skipped:     %d\n", result.RoomsSkipped)
	fmt.Printf("  Messages migrated: %d\n", result.MessagesMigrated)
	fmt.Printf("  Agents created:    %d\n", result.AgentsCreated)
	fmt.Printf("  Agents existing:   %d\n", result.AgentsExisting)
}
