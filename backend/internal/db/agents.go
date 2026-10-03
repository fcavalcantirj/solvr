package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// Agent-related errors.
var (
	ErrDuplicateAgentID    = errors.New("agent ID already exists")
	ErrAgentNotFound       = errors.New("agent not found")
	ErrAgentAlreadyClaimed = errors.New("agent is already claimed by a human")
	ErrDuplicateAMCPAID    = errors.New("amcp_aid already in use by another agent")
)

// AgentRepository handles database operations for agents.
// Per SPEC.md Part 6: agents table.
type AgentRepository struct {
	pool *Pool
}

// agentColumns defines the standard columns returned when querying agents.
// Used to keep queries consistent and DRY.
// Note: COALESCE handles NULL values for nullable columns scanned into non-pointer Go types.
// Without COALESCE, pgx fails when scanning NULL into string/[]string.
// 25 columns total (added keri_public_key for KERI identity management)
const agentColumns = `id, display_name, human_id, COALESCE(bio, '') as bio, COALESCE(specialties, '{}') as specialties, COALESCE(avatar_url, '') as avatar_url, COALESCE(api_key_hash, '') as api_key_hash, COALESCE(moltbook_id, '') as moltbook_id, COALESCE(model, '') as model, COALESCE(email, '') as email, COALESCE(external_links, '{}') as external_links, status, reputation, human_claimed_at, has_human_backed_badge, has_amcp_identity, COALESCE(amcp_aid, '') as amcp_aid, COALESCE(keri_public_key, '') as keri_public_key, pinning_quota_bytes, storage_used_bytes, last_seen_at, last_briefing_at, created_at, updated_at, deleted_at`

// NewAgentRepository creates a new AgentRepository.
func NewAgentRepository(pool *Pool) *AgentRepository {
	return &AgentRepository{pool: pool}
}

// Create inserts a new agent into the database.
// The agent struct is populated with timestamps after successful creation.
func (r *AgentRepository) Create(ctx context.Context, agent *models.Agent) error {
	query := `
		INSERT INTO agents (id, display_name, human_id, bio, specialties, avatar_url, api_key_hash, key_sha256, moltbook_id, model, email, external_links, has_amcp_identity, amcp_aid, keri_public_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING ` + agentColumns

	// Convert empty AMCP AID to nil for nullable column
	var amcpAID *string
	if agent.AMCPAID != "" {
		amcpAID = &agent.AMCPAID
	}

	// Convert empty KERI public key to nil for nullable column
	var keriPubKey *string
	if agent.KERIPublicKey != "" {
		keriPubKey = &agent.KERIPublicKey
	}

	var keySHA256 *string
	if agent.KeySHA256 != "" {
		keySHA256 = &agent.KeySHA256
	}

	row := r.pool.QueryRow(ctx, query,
		agent.ID,
		agent.DisplayName,
		agent.HumanID,
		agent.Bio,
		agent.Specialties,
		agent.AvatarURL,
		agent.APIKeyHash,
		keySHA256,
		agent.MoltbookID,
		agent.Model,
		agent.Email,
		agent.ExternalLinks,
		agent.HasAMCPIdentity,
		amcpAID,
		keriPubKey,
	)

	err := row.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.HasAMCPIdentity,
		&agent.AMCPAID,
		&agent.KERIPublicKey,
		&agent.PinningQuotaBytes,
		&agent.StorageUsedBytes,
		&agent.LastSeenAt,
		&agent.LastBriefingAt,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.DeletedAt,
	)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			slog.Info("duplicate key constraint", "op", "Create", "table", "agents", "constraint", "id", "agent_id", agent.ID)
			return ErrDuplicateAgentID
		}
		LogQueryError(ctx, "Create", "agents", err)
		return err
	}

	return nil
}

// FindByID finds an agent by their ID.
// Filters out soft-deleted agents (WHERE deleted_at IS NULL).
func (r *AgentRepository) FindByID(ctx context.Context, id string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, query, id)
	return r.scanAgent(row)
}

// FindByHumanID finds all agents owned by a human user.
// Filters out soft-deleted agents (WHERE deleted_at IS NULL).
func (r *AgentRepository) FindByHumanID(ctx context.Context, humanID string) ([]*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE human_id = $1 AND deleted_at IS NULL ORDER BY reputation DESC, created_at DESC`

	rows, err := r.pool.Query(ctx, query, humanID)
	if err != nil {
		LogQueryError(ctx, "FindByHumanID", "agents", err)
		return nil, err
	}
	defer rows.Close()

	var agents []*models.Agent
	for rows.Next() {
		agent, err := r.scanAgentRows(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}

	return agents, rows.Err()
}

// FindByAPIKeyHash finds an agent by their API key hash.
// Used for API key authentication.
// Filters out soft-deleted agents (WHERE deleted_at IS NULL).
// This ensures deleted agents cannot authenticate via API key.
func (r *AgentRepository) FindByAPIKeyHash(ctx context.Context, hash string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE api_key_hash = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, query, hash)
	return r.scanAgent(row)
}

// Update updates an existing agent.
// Updates display_name, bio, specialties, avatar_url, model, email, external_links.
// The agent struct is updated with new values after successful update.
func (r *AgentRepository) Update(ctx context.Context, agent *models.Agent) error {
	query := `
		UPDATE agents
		SET display_name = $2, bio = $3, specialties = $4, avatar_url = $5, model = $6, email = $7, external_links = $8, updated_at = NOW()
		WHERE id = $1
		RETURNING ` + agentColumns

	row := r.pool.QueryRow(ctx, query,
		agent.ID,
		agent.DisplayName,
		agent.Bio,
		agent.Specialties,
		agent.AvatarURL,
		agent.Model,
		agent.Email,
		agent.ExternalLinks,
	)

	err := row.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.HasAMCPIdentity,
		&agent.AMCPAID,
		&agent.KERIPublicKey,
		&agent.PinningQuotaBytes,
		&agent.StorageUsedBytes,
		&agent.LastSeenAt,
		&agent.LastBriefingAt,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Debug("agent not found", "op", "Update", "table", "agents", "id", agent.ID)
			return ErrAgentNotFound
		}
		LogQueryError(ctx, "Update", "agents", err)
		return err
	}

	return nil
}

// Delete soft-deletes an agent by setting deleted_at timestamp.
// Per PRD-v5 Task 22: agent self-deletion feature.
// This is a soft delete - the agent record remains in the database but is hidden from queries.
// Posts created by the agent remain visible (no cascade delete).
// Returns ErrAgentNotFound if agent doesn't exist or is already deleted.
func (r *AgentRepository) Delete(ctx context.Context, id string) error {
	query := `
		UPDATE agents
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		LogQueryError(ctx, "Delete", "agents", err)
		return fmt.Errorf("failed to delete agent: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAgentNotFound
	}

	return nil
}

// ListDeleted returns soft-deleted agents with pagination.
// Per PRD-v5 Task 17: Admin endpoints to review deleted accounts before permanent deletion.
// Returns agents ordered by deleted_at DESC (most recently deleted first).
func (r *AgentRepository) ListDeleted(ctx context.Context, page, perPage int) ([]models.Agent, int, error) {
	offset := (page - 1) * perPage

	query := `SELECT ` + agentColumns + ` FROM agents WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC LIMIT $1 OFFSET $2`

	rows, err := r.pool.Query(ctx, query, perPage, offset)
	if err != nil {
		LogQueryError(ctx, "ListDeleted", "agents", err)
		return nil, 0, err
	}
	defer rows.Close()

	var agents []models.Agent
	for rows.Next() {
		agent, err := r.scanAgentRows(rows)
		if err != nil {
			LogQueryError(ctx, "ListDeleted.Scan", "agents", err)
			return nil, 0, err
		}
		agents = append(agents, *agent)
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "ListDeleted.Rows", "agents", err)
		return nil, 0, err
	}

	// Count total deleted agents
	var total int
	countQuery := `SELECT COUNT(*) FROM agents WHERE deleted_at IS NOT NULL`
	err = r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		LogQueryError(ctx, "ListDeleted.Count", "agents", err)
		return nil, 0, err
	}

	return agents, total, nil
}

// UpdateAPIKeyHash updates the API key hash and SHA256 for an agent.
// Used when regenerating API keys.
func (r *AgentRepository) UpdateAPIKeyHash(ctx context.Context, agentID, hash, keySHA256 string) error {
	query := `
		UPDATE agents
		SET api_key_hash = $2, key_sha256 = $3, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, agentID, hash, keySHA256)
	if err != nil {
		LogQueryError(ctx, "UpdateAPIKeyHash", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("agent not found", "op", "UpdateAPIKeyHash", "table", "agents", "id", agentID)
		return ErrAgentNotFound
	}

	return nil
}

// RevokeAPIKey sets the API key hash to NULL, effectively revoking the key.
func (r *AgentRepository) RevokeAPIKey(ctx context.Context, agentID string) error {
	query := `
		UPDATE agents
		SET api_key_hash = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.pool.Exec(ctx, query, agentID)
	if err != nil {
		LogQueryError(ctx, "RevokeAPIKey", "agents", err)
		return err
	}

	if result.RowsAffected() == 0 {
		slog.Debug("agent not found", "op", "RevokeAPIKey", "table", "agents", "id", agentID)
		return ErrAgentNotFound
	}

	return nil
}

// scanAgent scans an agent row into an Agent struct.
// Expects columns in order defined by agentColumns constant (25 columns).
func (r *AgentRepository) scanAgent(row pgx.Row) (*models.Agent, error) {
	agent := &models.Agent{}
	err := row.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.HasAMCPIdentity,
		&agent.AMCPAID,
		&agent.KERIPublicKey,
		&agent.PinningQuotaBytes,
		&agent.StorageUsedBytes,
		&agent.LastSeenAt,
		&agent.LastBriefingAt,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.DeletedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}

	return agent, nil
}

// scanAgentRows scans a rows result into an Agent struct.
// Used for queries that return multiple rows (25 columns).
func (r *AgentRepository) scanAgentRows(rows pgx.Rows) (*models.Agent, error) {
	agent := &models.Agent{}
	err := rows.Scan(
		&agent.ID,
		&agent.DisplayName,
		&agent.HumanID,
		&agent.Bio,
		&agent.Specialties,
		&agent.AvatarURL,
		&agent.APIKeyHash,
		&agent.MoltbookID,
		&agent.Model,
		&agent.Email,
		&agent.ExternalLinks,
		&agent.Status,
		&agent.Reputation,
		&agent.HumanClaimedAt,
		&agent.HasHumanBackedBadge,
		&agent.HasAMCPIdentity,
		&agent.AMCPAID,
		&agent.KERIPublicKey,
		&agent.PinningQuotaBytes,
		&agent.StorageUsedBytes,
		&agent.LastSeenAt,
		&agent.LastBriefingAt,
		&agent.CreatedAt,
		&agent.UpdatedAt,
		&agent.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return agent, nil
}

// UpdateIdentity updates AMCP identity fields (amcp_aid, keri_public_key) for an agent.
// Supports partial updates: only provided (non-nil) fields are changed.
// Returns ErrDuplicateAMCPAID if the amcp_aid is already used by another agent.
// Returns the updated agent.
func (r *AgentRepository) UpdateIdentity(ctx context.Context, agentID string, amcpAID *string, keriPublicKey *string) (*models.Agent, error) {
	// Build dynamic SET clause for partial update
	setClauses := []string{"updated_at = NOW()"}
	args := []any{agentID}
	argNum := 2

	if amcpAID != nil {
		setClauses = append(setClauses, fmt.Sprintf("amcp_aid = $%d", argNum))
		if *amcpAID == "" {
			args = append(args, nil)
		} else {
			args = append(args, *amcpAID)
		}
		argNum++
		// Set has_amcp_identity based on whether amcp_aid is non-empty
		setClauses = append(setClauses, fmt.Sprintf("has_amcp_identity = $%d", argNum))
		args = append(args, *amcpAID != "")
		argNum++
		// Auto-provision pinning quota for AMCP agents (1 GB)
		if *amcpAID != "" {
			setClauses = append(setClauses, fmt.Sprintf("pinning_quota_bytes = GREATEST(pinning_quota_bytes, $%d)", argNum))
			args = append(args, int64(1073741824))
			argNum++
		}
	}

	if keriPublicKey != nil {
		setClauses = append(setClauses, fmt.Sprintf("keri_public_key = $%d", argNum))
		if *keriPublicKey == "" {
			args = append(args, nil)
		} else {
			args = append(args, *keriPublicKey)
		}
		argNum++
	}

	query := `UPDATE agents SET ` + strings.Join(setClauses, ", ") + ` WHERE id = $1 AND deleted_at IS NULL RETURNING ` + agentColumns

	row := r.pool.QueryRow(ctx, query, args...)
	agent, err := r.scanAgent(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrAgentNotFound) {
			return nil, ErrAgentNotFound
		}
		// Check for unique constraint violation on amcp_aid or keri_public_key
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			if strings.Contains(err.Error(), "amcp_aid") {
				return nil, ErrDuplicateAMCPAID
			}
			if strings.Contains(err.Error(), "keri_public_key") {
				return nil, errors.New("keri_public_key already in use by another agent")
			}
		}
		LogQueryError(ctx, "UpdateIdentity", "agents", err)
		return nil, err
	}

	return agent, nil
}

// FindByName finds an agent by their display name.
// Used for name uniqueness checks during registration.
// Filters out soft-deleted agents (WHERE deleted_at IS NULL).
func (r *AgentRepository) FindByName(ctx context.Context, name string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE display_name = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, query, name)
	return r.scanAgent(row)
}

// GetAgentByAPIKeyHash finds an agent by validating the raw API key against stored hashes.
// Uses SHA256 for O(1) indexed lookup. Falls back to O(n) bcrypt scan for agents
// that haven't been backfilled yet, and lazy-backfills their SHA256 on match.
// Returns the matching agent if found, or (nil, nil) if no agent matches.
func (r *AgentRepository) GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error) {
	keySHA256 := auth.SHA256APIKey(key)

	// Fast path: O(1) lookup by SHA256 index
	agent, err := r.getAgentByKeySHA256(ctx, keySHA256)
	if err != nil {
		return nil, err
	}
	if agent != nil {
		return agent, nil
	}

	// Slow path: bcrypt scan for agents without SHA256 (lazy backfill)
	return r.getAgentByKeyBcryptFallback(ctx, key, keySHA256)
}

// getAgentByKeySHA256 does an O(1) indexed lookup by SHA256 hash.
func (r *AgentRepository) getAgentByKeySHA256(ctx context.Context, keySHA256 string) (*models.Agent, error) {
	query := `SELECT ` + agentColumns + ` FROM agents WHERE key_sha256 = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, query, keySHA256)
	agent, err := r.scanAgent(row)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return nil, nil // Not found via SHA256, try fallback
		}
		return nil, err
	}
	return agent, nil
}

// getAgentByKeyBcryptFallback scans agents without SHA256 and compares via bcrypt.
// On match, lazy-backfills the SHA256 for future O(1) lookups.
func (r *AgentRepository) getAgentByKeyBcryptFallback(ctx context.Context, key, keySHA256 string) (*models.Agent, error) {
	// Cap the total time spent on bcrypt fallback scanning to prevent CPU spin
	// on invalid keys that would otherwise compare against every row.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	query := `SELECT ` + agentColumns + ` FROM agents WHERE api_key_hash IS NOT NULL AND api_key_hash != '' AND deleted_at IS NULL AND key_sha256 IS NULL`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "getAgentByKeyBcryptFallback", "agents", err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		if ctx.Err() != nil {
			rows.Close()
			return nil, nil
		}

		agent, err := r.scanAgentRows(rows)
		if err != nil {
			return nil, err
		}

		if err := bcrypt.CompareHashAndPassword([]byte(agent.APIKeyHash), []byte(key)); err == nil {
			// Match found — lazy backfill SHA256
			r.backfillAgentKeySHA256(ctx, agent.ID, keySHA256)
			return agent, nil
		}
	}

	if err := rows.Err(); err != nil {
		LogQueryError(ctx, "getAgentByKeyBcryptFallback.Rows", "agents", err)
		return nil, err
	}

	return nil, nil
}

// backfillAgentKeySHA256 stores the SHA256 hash for an agent key matched via bcrypt.
func (r *AgentRepository) backfillAgentKeySHA256(ctx context.Context, agentID, keySHA256 string) {
	query := `UPDATE agents SET key_sha256 = $1 WHERE id = $2 AND key_sha256 IS NULL`
	if _, err := r.pool.Exec(ctx, query, keySHA256, agentID); err != nil {
		slog.Warn("failed to backfill agent key_sha256", "agentID", agentID, "error", err)
	}
}

// CountActive returns the total number of active agents.
// Filters out soft-deleted agents (WHERE deleted_at IS NULL).
func (r *AgentRepository) CountActive(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM agents WHERE status = 'active' AND deleted_at IS NULL`
	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountActive", "agents", err)
		return 0, err
	}
	return count, nil
}

// CountHumanBacked returns the total number of agents with human-backed badge.
// Filters out soft-deleted agents (WHERE deleted_at IS NULL).
// UpdateLastSeen sets the last_seen_at timestamp to NOW() for heartbeat liveness tracking.
func (r *AgentRepository) UpdateLastSeen(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE agents SET last_seen_at = NOW() WHERE id = $1`, id)
	if err != nil {
		LogQueryError(ctx, "UpdateLastSeen", "agents", err)
	}
	return err
}

// UpdateLastBriefingAt sets the last_briefing_at timestamp to NOW() when agent calls GET /me.
// Used for delta calculations (new notifications, reputation changes since last briefing).
func (r *AgentRepository) UpdateLastBriefingAt(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE agents SET last_briefing_at = NOW() WHERE id = $1`, id)
	if err != nil {
		LogQueryError(ctx, "UpdateLastBriefingAt", "agents", err)
	}
	return err
}

// GetLastBriefingAt returns the last_briefing_at timestamp for an agent.
// Returns nil if the agent has never called GET /me.
func (r *AgentRepository) GetLastBriefingAt(ctx context.Context, id string) (*time.Time, error) {
	var lastBriefingAt *time.Time
	err := r.pool.QueryRow(ctx, `SELECT last_briefing_at FROM agents WHERE id = $1`, id).Scan(&lastBriefingAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentNotFound
		}
		LogQueryError(ctx, "GetLastBriefingAt", "agents", err)
		return nil, err
	}
	return lastBriefingAt, nil
}

func (r *AgentRepository) CountHumanBacked(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM agents WHERE has_human_backed_badge = true AND deleted_at IS NULL`
	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountHumanBacked", "agents", err)
		return 0, err
	}
	return count, nil
}
