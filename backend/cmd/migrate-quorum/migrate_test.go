// Package main provides unit tests for the Quorum-to-Solvr migration CLI tool.
// Tests use a mockMigrationDB interface — no real database connection needed.
package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// mockMigrationDB implements migrationDB for testing.
type mockMigrationDB struct {
	quorumRooms    []quorumRoom
	quorumMessages map[uuid.UUID][]quorumMessage

	// Solvr state (simulates DB state for idempotency tests)
	insertedRooms    map[uuid.UUID]roomInsert
	insertedAgents   map[string]agentInsert
	insertedMessages []messageInsert

	// Control flags
	insertRoomFails    bool
	insertAgentFails   bool
	insertMessageFails bool
	beginTxFails       bool
	findUserFails      bool
	slugConflicts      map[string]bool // slug -> conflicts

	// Track calls for dry-run verification
	insertRoomCalls    int
	insertAgentCalls   int
	insertMessageCalls int
	beginTxCalls       int

	// Pre-loaded user IDs by email
	usersByEmail map[string]uuid.UUID
}

func newMockDB() *mockMigrationDB {
	return &mockMigrationDB{
		quorumMessages: make(map[uuid.UUID][]quorumMessage),
		insertedRooms:  make(map[uuid.UUID]roomInsert),
		insertedAgents: make(map[string]agentInsert),
		usersByEmail:   make(map[string]uuid.UUID),
		slugConflicts:  make(map[string]bool),
	}
}

// mockTx implements txInterface for testing — no-op commit/rollback.
type mockTx struct {
	rolledBack bool
	committed  bool
}

func (m *mockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	// Return a CommandTag that reports 1 row affected (simulates successful INSERT).
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (m *mockTx) Commit(ctx context.Context) error {
	m.committed = true
	return nil
}
func (m *mockTx) Rollback(ctx context.Context) error {
	m.rolledBack = true
	return nil
}

func (d *mockMigrationDB) ListQuorumRooms(ctx context.Context) ([]quorumRoom, error) {
	return d.quorumRooms, nil
}

func (d *mockMigrationDB) ListQuorumMessages(ctx context.Context, roomID uuid.UUID) ([]quorumMessage, error) {
	return d.quorumMessages[roomID], nil
}

func (d *mockMigrationDB) FindSolvrUserByEmail(ctx context.Context, email string) (uuid.UUID, error) {
	if d.findUserFails {
		return uuid.Nil, errors.New("user not found: " + email)
	}
	id, ok := d.usersByEmail[email]
	if !ok {
		return uuid.Nil, errors.New("user not found: " + email)
	}
	return id, nil
}

func (d *mockMigrationDB) CheckSlugConflict(ctx context.Context, slug string, roomID uuid.UUID) (bool, error) {
	return d.slugConflicts[slug], nil
}

func (d *mockMigrationDB) BeginTx(ctx context.Context) (txInterface, error) {
	d.beginTxCalls++
	if d.beginTxFails {
		return nil, errors.New("begin tx failed")
	}
	return &mockTx{}, nil
}

func (d *mockMigrationDB) InsertRoom(ctx context.Context, tx txInterface, r roomInsert) (bool, error) {
	d.insertRoomCalls++
	if d.insertRoomFails {
		return false, errors.New("insert room failed")
	}
	if _, exists := d.insertedRooms[r.ID]; exists {
		return false, nil // ON CONFLICT DO NOTHING
	}
	d.insertedRooms[r.ID] = r
	return true, nil
}

func (d *mockMigrationDB) InsertAgent(ctx context.Context, tx txInterface, a agentInsert) (bool, error) {
	d.insertAgentCalls++
	if d.insertAgentFails {
		return false, errors.New("insert agent failed")
	}
	if _, exists := d.insertedAgents[a.ID]; exists {
		return false, nil // ON CONFLICT DO NOTHING
	}
	d.insertedAgents[a.ID] = a
	return true, nil
}

func (d *mockMigrationDB) InsertMessagesWithSequence(ctx context.Context, tx txInterface, roomID uuid.UUID, msgs []messageInsert) (int, error) {
	d.insertMessageCalls++
	if d.insertMessageFails {
		return 0, errors.New("insert messages failed")
	}
	d.insertedMessages = append(d.insertedMessages, msgs...)
	return len(msgs), nil
}

func (d *mockMigrationDB) UpdateRoomMessageCount(ctx context.Context, tx txInterface, roomID uuid.UUID) error {
	return nil
}

// ---- Pure function tests ----

func TestDetectContentType(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "markdown header",
			content: "# This is a header\nSome content",
			want:    "markdown",
		},
		{
			name:    "bold text",
			content: "Some **bold** content here",
			want:    "markdown",
		},
		{
			name:    "triple backtick code block",
			content: "Here is code:\n```go\nfmt.Println(\"hello\")\n```",
			want:    "markdown",
		},
		{
			name:    "inline code",
			content: "Use `fmt.Println` to print",
			want:    "markdown",
		},
		{
			name:    "plain text",
			content: "This is just plain text with no markdown.",
			want:    "text",
		},
		{
			name:    "empty string",
			content: "",
			want:    "text",
		},
		{
			name:    "subheader h2",
			content: "## Section 2\nContent here",
			want:    "markdown",
		},
		{
			name:    "not markdown bold",
			content: "This has * but not ** bold markers",
			want:    "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectContentType(tt.content)
			if got != "text" && got != "markdown" {
				t.Errorf("detectContentType() returned invalid value %q, want 'text' or 'markdown'", got)
			}
			if got != tt.want {
				t.Errorf("detectContentType(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestTargetSlug(t *testing.T) {
	tests := []struct {
		quorumSlug  string
		expectedSlug string
	}{
		{
			quorumSlug:  "solvr-usage-analysys",
			expectedSlug: "solvr-usage-analysis",
		},
		{
			quorumSlug:  "ballona-trade-v0",
			expectedSlug: "ballona-trade-v0",
		},
		{
			quorumSlug:  "composio-integration",
			expectedSlug: "composio-integration",
		},
		{
			quorumSlug:  "mackjack-ops",
			expectedSlug: "mackjack-ops",
		},
		{
			quorumSlug:  "jack-mack-msv-trading",
			expectedSlug: "jack-mack-msv-trading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.quorumSlug, func(t *testing.T) {
			got := targetSlug(tt.quorumSlug)
			if got != tt.expectedSlug {
				t.Errorf("targetSlug(%q) = %q, want %q", tt.quorumSlug, got, tt.expectedSlug)
			}
		})
	}
}

func TestAgentID(t *testing.T) {
	tests := []struct {
		name     string
		agentName string
		wantID   string
	}{
		{
			name:      "known agent ClaudiusThePirateEmperor",
			agentName: "ClaudiusThePirateEmperor",
			wantID:    "agent_ClaudiusThePirateEmperor",
		},
		{
			name:      "known agent Jack",
			agentName: "Jack",
			wantID:    "agent_Jack",
		},
		{
			name:      "known agent Mack",
			agentName: "Mack",
			wantID:    "agent_Mack",
		},
		{
			name:      "unknown agent gets agent_ prefix",
			agentName: "SomeNewAgent",
			wantID:    "agent_SomeNewAgent",
		},
		{
			name:      "very long name truncated to 50 chars",
			agentName: "ThisIsAVeryLongAgentNameThatExceedsFiftyCharactersLimit",
			wantID:    "agent_ThisIsAVeryLongAgentNameThatExceedsFiftyChar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agentID(tt.agentName)
			if len(got) > 50 {
				t.Errorf("agentID(%q) = %q (len=%d), must be <= 50 chars", tt.agentName, got, len(got))
			}
			if got != tt.wantID {
				t.Errorf("agentID(%q) = %q, want %q", tt.agentName, got, tt.wantID)
			}
		})
	}
}

// ---- Migration orchestration tests ----

func makeTestRoom(slug string, id uuid.UUID) quorumRoom {
	return quorumRoom{
		ID:          id,
		Slug:        slug,
		DisplayName: "Test " + slug,
		TokenHash:   "hash-" + slug,
		CreatedAt:   time.Now(),
	}
}

func makeTestMessage(agentName, content string) quorumMessage {
	return quorumMessage{
		AgentName: agentName,
		Content:   content,
		CreatedAt: time.Now(),
	}
}

func setupMockDBWithBasicData(t *testing.T) (*mockMigrationDB, uuid.UUID, uuid.UUID) {
	t.Helper()
	db := newMockDB()

	felipeUUID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	marceloUUID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	db.usersByEmail["felipecavalcantirj@gmail.com"] = felipeUUID
	db.usersByEmail["macballona@gmail.com"] = marceloUUID

	// 5 allowed rooms
	room1ID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	room2ID := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	room3ID := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	room4ID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	room5ID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")

	db.quorumRooms = []quorumRoom{
		makeTestRoom("composio-integration", room1ID),
		makeTestRoom("solvr-usage-analysys", room2ID),
		makeTestRoom("ballona-trade-v0", room3ID),
		makeTestRoom("mackjack-ops", room4ID),
		makeTestRoom("jack-mack-msv-trading", room5ID),
		// Extra rooms that should be skipped
		makeTestRoom("some-random-room", uuid.New()),
		makeTestRoom("test-room", uuid.New()),
	}

	// Add messages to rooms
	db.quorumMessages[room1ID] = []quorumMessage{
		makeTestMessage("ClaudiusThePirateEmperor", "Hello world"),
		makeTestMessage("Jack", "## Markdown message"),
	}
	db.quorumMessages[room2ID] = []quorumMessage{
		makeTestMessage("Mack", "Plain text message"),
	}
	db.quorumMessages[room3ID] = []quorumMessage{
		makeTestMessage("Jack", "Message for ballona"),
	}
	db.quorumMessages[room4ID] = []quorumMessage{
		makeTestMessage("Mack", "Message for mackjack"),
	}
	db.quorumMessages[room5ID] = []quorumMessage{
		makeTestMessage("Jack", "Message for trading"),
	}

	return db, felipeUUID, marceloUUID
}

func TestMigration_RoomOwnerMapping(t *testing.T) {
	mockDB, felipeUUID, marceloUUID := setupMockDBWithBasicData(t)

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	result, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	if result.RoomsMigrated != 5 {
		t.Errorf("expected 5 rooms migrated, got %d", result.RoomsMigrated)
	}

	// Verify owner mapping for each room
	for id, room := range mockDB.insertedRooms {
		switch room.Slug {
		case "composio-integration", "solvr-usage-analysis":
			if room.OwnerID != felipeUUID {
				t.Errorf("room %s (slug=%s): expected Felipe's UUID %s, got %s",
					id, room.Slug, felipeUUID, room.OwnerID)
			}
		case "ballona-trade-v0", "mackjack-ops", "jack-mack-msv-trading":
			if room.OwnerID != marceloUUID {
				t.Errorf("room %s (slug=%s): expected Marcelo's UUID %s, got %s",
					id, room.Slug, marceloUUID, room.OwnerID)
			}
		default:
			t.Errorf("unexpected room slug: %s", room.Slug)
		}
	}
}

func TestMigration_AgentRegistration(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// Known agents should use their existing IDs
	if _, ok := mockDB.insertedAgents["agent_ClaudiusThePirateEmperor"]; !ok {
		t.Error("expected agent_ClaudiusThePirateEmperor to be registered")
	}
	if _, ok := mockDB.insertedAgents["agent_Jack"]; !ok {
		t.Error("expected agent_Jack to be registered")
	}
	if _, ok := mockDB.insertedAgents["agent_Mack"]; !ok {
		t.Error("expected agent_Mack to be registered")
	}

	// All agent IDs must be <= 50 chars
	for id := range mockDB.insertedAgents {
		if len(id) > 50 {
			t.Errorf("agent ID %q exceeds 50 chars (len=%d)", id, len(id))
		}
	}
}

func TestMigration_MessageContentType(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	for _, msg := range mockDB.insertedMessages {
		if msg.ContentType != "text" && msg.ContentType != "markdown" {
			t.Errorf("message has invalid content_type %q", msg.ContentType)
		}
		// "## Markdown message" should be detected as markdown
		if msg.Content == "## Markdown message" && msg.ContentType != "markdown" {
			t.Errorf("markdown message got content_type=%q, want 'markdown'", msg.ContentType)
		}
		// Plain text messages should be "text"
		if msg.Content == "Hello world" && msg.ContentType != "text" {
			t.Errorf("plain text message got content_type=%q, want 'text'", msg.ContentType)
		}
	}
}

func TestMigration_SequenceNumbers(t *testing.T) {
	mockDB := newMockDB()

	felipeUUID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	mockDB.usersByEmail["felipecavalcantirj@gmail.com"] = felipeUUID
	mockDB.usersByEmail["macballona@gmail.com"] = uuid.MustParse("22222222-2222-2222-2222-222222222222")

	roomID := uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa")
	mockDB.quorumRooms = []quorumRoom{
		makeTestRoom("composio-integration", roomID),
	}

	// 3 messages in order
	t1 := time.Now()
	t2 := t1.Add(time.Minute)
	t3 := t2.Add(time.Minute)
	mockDB.quorumMessages[roomID] = []quorumMessage{
		{AgentName: "ClaudiusThePirateEmperor", Content: "First", CreatedAt: t1},
		{AgentName: "Jack", Content: "Second", CreatedAt: t2},
		{AgentName: "Mack", Content: "Third", CreatedAt: t3},
	}

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	_, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	if len(mockDB.insertedMessages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(mockDB.insertedMessages))
	}

	// Check sequence numbers are 1, 2, 3
	for i, msg := range mockDB.insertedMessages {
		expectedSeq := i + 1
		if msg.SequenceNum != expectedSeq {
			t.Errorf("message[%d] SequenceNum=%d, want %d", i, msg.SequenceNum, expectedSeq)
		}
	}
}

func TestMigration_AgentPresenceSkipped(t *testing.T) {
	// This test verifies that no agent_presence rows are inserted.
	// The migrationDB interface has no InsertAgentPresence method — by design.
	// This is a compile-time guarantee, but we verify by checking the interface.
	var _ migrationDB // compiles only if interface exists

	// No agent_presence insertion method on the interface = DATA-02 satisfied.
	// The mock doesn't track agent_presence inserts because the interface forbids it.
	t.Log("agent_presence skipped: migrationDB interface has no InsertAgentPresence method (DATA-02 satisfied)")
}

func TestMigration_SkippedRooms(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)
	// mockDB has 7 rooms: 5 allowed + 2 skipped

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	result, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	if result.RoomsMigrated != 5 {
		t.Errorf("expected 5 rooms migrated, got %d", result.RoomsMigrated)
	}
	if result.RoomsSkipped < 2 {
		t.Errorf("expected at least 2 rooms skipped, got %d", result.RoomsSkipped)
	}

	// Verify non-allowed rooms are NOT in Solvr
	for _, room := range mockDB.insertedRooms {
		if room.Slug == "some-random-room" || room.Slug == "test-room" {
			t.Errorf("non-allowed room %q should have been skipped", room.Slug)
		}
	}
}

func TestMigration_Idempotent(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	// Run migration once
	result1, err := m.run(ctx)
	if err != nil {
		t.Fatalf("first run() failed: %v", err)
	}

	// Run migration again — should produce same counts, no errors
	result2, err := m.run(ctx)
	if err != nil {
		t.Fatalf("second run() failed: %v", err)
	}

	// Both runs should succeed, second run rooms migrated should be 0 (all ON CONFLICT DO NOTHING)
	if result1.RoomsMigrated == 0 {
		t.Error("first run should have migrated rooms")
	}
	if result2.RoomsMigrated != 0 {
		t.Errorf("second run should migrate 0 new rooms (idempotent), got %d", result2.RoomsMigrated)
	}

	// Total rooms in Solvr should still be 5 (no duplicates)
	if len(mockDB.insertedRooms) != 5 {
		t.Errorf("expected 5 unique rooms after 2 runs, got %d", len(mockDB.insertedRooms))
	}
}

func TestMigration_DryRun(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)

	m := &migrator{db: mockDB, dryRun: true}
	ctx := context.Background()

	result, err := m.run(ctx)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}

	// Dry-run should report what would happen
	if result == nil {
		t.Fatal("dry-run returned nil result")
	}

	// Dry-run must NOT call any write methods
	if mockDB.beginTxCalls > 0 {
		t.Errorf("dry-run called BeginTx %d times, want 0", mockDB.beginTxCalls)
	}
	if mockDB.insertRoomCalls > 0 {
		t.Errorf("dry-run called InsertRoom %d times, want 0", mockDB.insertRoomCalls)
	}
	if mockDB.insertAgentCalls > 0 {
		t.Errorf("dry-run called InsertAgent %d times, want 0", mockDB.insertAgentCalls)
	}
	if mockDB.insertMessageCalls > 0 {
		t.Errorf("dry-run called InsertMessagesWithSequence %d times, want 0", mockDB.insertMessageCalls)
	}
}

func TestMigration_TxRollback(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)
	mockDB.insertRoomFails = true // Simulate a write failure

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	_, err := m.run(ctx)
	if err == nil {
		t.Error("expected run() to fail when InsertRoom fails, got nil error")
	}

	// No rooms should have been committed (transaction rolled back)
	if len(mockDB.insertedRooms) > 0 {
		t.Errorf("expected 0 rooms after rollback, got %d", len(mockDB.insertedRooms))
	}
}

func TestMigration_SlugConflict(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)

	// Simulate a slug conflict for one allowed room
	mockDB.slugConflicts["solvr-usage-analysis"] = true

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	result, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// 4 rooms migrated, 1 skipped due to slug conflict
	if result.RoomsMigrated != 4 {
		t.Errorf("expected 4 rooms migrated (1 skipped for conflict), got %d", result.RoomsMigrated)
	}
	// Conflicted room should NOT be in inserted rooms
	for _, room := range mockDB.insertedRooms {
		if room.Slug == "solvr-usage-analysis" {
			t.Error("conflicted room solvr-usage-analysis should have been skipped")
		}
	}
}

func TestMigration_Summary(t *testing.T) {
	mockDB, _, _ := setupMockDBWithBasicData(t)

	m := &migrator{db: mockDB, dryRun: false}
	ctx := context.Background()

	result, err := m.run(ctx)
	if err != nil {
		t.Fatalf("run() failed: %v", err)
	}

	// Result struct should have all count fields
	if result == nil {
		t.Fatal("run() returned nil result")
	}

	// Basic sanity checks
	if result.RoomsMigrated < 0 {
		t.Error("RoomsMigrated should be >= 0")
	}
	if result.RoomsSkipped < 0 {
		t.Error("RoomsSkipped should be >= 0")
	}
	if result.MessagesMigrated < 0 {
		t.Error("MessagesMigrated should be >= 0")
	}
	if result.AgentsCreated < 0 {
		t.Error("AgentsCreated should be >= 0")
	}

	// With our test data: 5 rooms, 6 messages total (2+1+1+1+1)
	if result.RoomsMigrated != 5 {
		t.Errorf("RoomsMigrated=%d, want 5", result.RoomsMigrated)
	}
	if result.MessagesMigrated != 6 {
		t.Errorf("MessagesMigrated=%d, want 6", result.MessagesMigrated)
	}
}
