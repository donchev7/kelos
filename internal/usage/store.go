package usage

import (
	"context"
	"embed"
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Store persists Cody usage facts into PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database URL: %w", err)
	}
	cfg.MaxConns = 5
	cfg.MinConns = 0
	cfg.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating PostgreSQL pool: %w", err)
	}
	store := &Store{pool: pool}
	if err := store.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("usage store is not initialized")
	}
	return s.pool.Ping(ctx)
}

func (s *Store) ApplyMigrations(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS cody_usage_schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`); err != nil {
		return fmt.Errorf("creating schema migrations table: %w", err)
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		version := strings.TrimSuffix(name, path.Ext(name))
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM cody_usage_schema_migrations WHERE version = $1)`, version).Scan(&exists); err != nil {
			return fmt.Errorf("checking migration %s: %w", version, err)
		}
		if exists {
			continue
		}
		sqlBytes, err := migrationFS.ReadFile(path.Join("migrations", name))
		if err != nil {
			return fmt.Errorf("reading migration %s: %w", name, err)
		}
		tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
		if err != nil {
			return fmt.Errorf("starting migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("running migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO cody_usage_schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("recording migration %s: %w", version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("committing migration %s: %w", version, err)
		}
	}
	return nil
}

func (s *Store) UpsertSession(ctx context.Context, record SessionRecord) error {
	if record.Persona == "" {
		record.Persona = "unknown"
	}
	if record.Source == "" {
		record.Source = sourceSlack
	}
	if record.Status == "" {
		record.Status = "active"
	}
	if record.FirstSeenAt.IsZero() {
		record.FirstSeenAt = time.Now().UTC()
	}
	if record.LastActivityAt.IsZero() {
		record.LastActivityAt = record.FirstSeenAt
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO cody_usage_sessions (
  session_id, cluster, namespace, taskspawner_name, persona, source,
  slack_team_id, slack_channel_id, slack_root_ts, first_user_id,
  first_seen_at, last_activity_at, first_task_name, agent_session_name, status
) VALUES (
  $1, $2, $3, $4, $5, $6,
  $7, $8, $9, $10,
  $11, $12, $13, $14, $15
)
ON CONFLICT (session_id) DO UPDATE SET
  cluster = EXCLUDED.cluster,
  namespace = EXCLUDED.namespace,
  taskspawner_name = EXCLUDED.taskspawner_name,
  persona = EXCLUDED.persona,
  source = CASE
    WHEN cody_usage_sessions.source = $16 AND EXCLUDED.source <> $16 THEN EXCLUDED.source
    ELSE cody_usage_sessions.source
  END,
  slack_team_id = COALESCE(cody_usage_sessions.slack_team_id, EXCLUDED.slack_team_id),
  slack_channel_id = COALESCE(cody_usage_sessions.slack_channel_id, EXCLUDED.slack_channel_id),
  slack_root_ts = COALESCE(cody_usage_sessions.slack_root_ts, EXCLUDED.slack_root_ts),
  first_user_id = COALESCE(cody_usage_sessions.first_user_id, EXCLUDED.first_user_id),
  first_seen_at = LEAST(cody_usage_sessions.first_seen_at, EXCLUDED.first_seen_at),
  last_activity_at = GREATEST(cody_usage_sessions.last_activity_at, EXCLUDED.last_activity_at),
  first_task_name = COALESCE(cody_usage_sessions.first_task_name, EXCLUDED.first_task_name),
  agent_session_name = COALESCE(cody_usage_sessions.agent_session_name, EXCLUDED.agent_session_name),
  status = EXCLUDED.status,
  updated_at = now()`,
		record.SessionID,
		record.Cluster,
		record.Namespace,
		record.TaskSpawnerName,
		record.Persona,
		record.Source,
		record.SlackTeamID,
		record.SlackChannelID,
		record.SlackRootTS,
		record.FirstUserID,
		record.FirstSeenAt,
		record.LastActivityAt,
		record.FirstTaskName,
		record.AgentSessionName,
		record.Status,
		sourceLoki,
	)
	recordResult("session", err)
	return err
}

func (s *Store) UpsertTurn(ctx context.Context, record TurnRecord) error {
	if record.Persona == "" {
		record.Persona = "unknown"
	}
	if record.Source == "" {
		record.Source = sourceSlack
	}
	if record.Phase == "" {
		record.Phase = "Pending"
	}
	_, err := s.pool.Exec(ctx, `
INSERT INTO cody_usage_turns (
  turn_id, session_id, cluster, namespace, taskspawner_name, persona, source,
  slack_user_id, slack_channel_id, slack_thread_ts, slack_message_ts,
  task_name, task_uid, agent_turn_name, agent_turn_uid,
  agent_type, model, phase, started_at, completed_at, duration_seconds,
  input_tokens, output_tokens, total_tokens, cost_usd, pr_url, error_message
) VALUES (
  $1, $2, $3, $4, $5, $6, $7,
  $8, $9, $10, $11,
  $12, $13, $14, $15,
  $16, $17, $18, $19, $20, $21,
  $22, $23, $24, $25, $26, $27
)
ON CONFLICT (turn_id) DO UPDATE SET
  session_id = EXCLUDED.session_id,
  cluster = EXCLUDED.cluster,
  namespace = EXCLUDED.namespace,
  taskspawner_name = EXCLUDED.taskspawner_name,
  persona = EXCLUDED.persona,
  source = CASE
    WHEN cody_usage_turns.source = $28 AND EXCLUDED.source <> $28 THEN EXCLUDED.source
    ELSE cody_usage_turns.source
  END,
  slack_user_id = COALESCE(cody_usage_turns.slack_user_id, EXCLUDED.slack_user_id),
  slack_channel_id = COALESCE(cody_usage_turns.slack_channel_id, EXCLUDED.slack_channel_id),
  slack_thread_ts = COALESCE(cody_usage_turns.slack_thread_ts, EXCLUDED.slack_thread_ts),
  slack_message_ts = COALESCE(cody_usage_turns.slack_message_ts, EXCLUDED.slack_message_ts),
  task_name = COALESCE(cody_usage_turns.task_name, EXCLUDED.task_name),
  task_uid = COALESCE(cody_usage_turns.task_uid, EXCLUDED.task_uid),
  agent_turn_name = COALESCE(cody_usage_turns.agent_turn_name, EXCLUDED.agent_turn_name),
  agent_turn_uid = COALESCE(cody_usage_turns.agent_turn_uid, EXCLUDED.agent_turn_uid),
  agent_type = COALESCE(cody_usage_turns.agent_type, EXCLUDED.agent_type),
  model = COALESCE(cody_usage_turns.model, EXCLUDED.model),
  phase = EXCLUDED.phase,
  started_at = COALESCE(cody_usage_turns.started_at, EXCLUDED.started_at),
  completed_at = COALESCE(EXCLUDED.completed_at, cody_usage_turns.completed_at),
  duration_seconds = COALESCE(EXCLUDED.duration_seconds, cody_usage_turns.duration_seconds),
  input_tokens = COALESCE(EXCLUDED.input_tokens, cody_usage_turns.input_tokens),
  output_tokens = COALESCE(EXCLUDED.output_tokens, cody_usage_turns.output_tokens),
  total_tokens = COALESCE(EXCLUDED.total_tokens, cody_usage_turns.total_tokens),
  cost_usd = COALESCE(EXCLUDED.cost_usd, cody_usage_turns.cost_usd),
  pr_url = COALESCE(EXCLUDED.pr_url, cody_usage_turns.pr_url),
  error_message = COALESCE(EXCLUDED.error_message, cody_usage_turns.error_message),
  updated_at = now()`,
		record.TurnID,
		record.SessionID,
		record.Cluster,
		record.Namespace,
		record.TaskSpawnerName,
		record.Persona,
		record.Source,
		record.SlackUserID,
		record.SlackChannelID,
		record.SlackThreadTS,
		record.SlackMessageTS,
		record.TaskName,
		record.TaskUID,
		record.AgentTurnName,
		record.AgentTurnUID,
		record.AgentType,
		record.Model,
		record.Phase,
		record.StartedAt,
		record.CompletedAt,
		record.DurationSeconds,
		record.InputTokens,
		record.OutputTokens,
		record.TotalTokens,
		record.CostUSD,
		record.PRURL,
		record.ErrorMessage,
		sourceLoki,
	)
	recordResult("turn", err)
	return err
}

func (s *Store) UpsertSessionAndTurn(ctx context.Context, session SessionRecord, turn TurnRecord) error {
	if err := requireRecordBasics(session, turn); err != nil {
		return err
	}
	if err := s.UpsertSession(ctx, session); err != nil {
		return err
	}
	return s.UpsertTurn(ctx, turn)
}

func (s *Store) SetOffset(ctx context.Context, source, cursor string) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO cody_usage_collector_offsets (source, cursor)
VALUES ($1, $2)
ON CONFLICT (source) DO UPDATE SET
  cursor = EXCLUDED.cursor,
  updated_at = now()`,
		source,
		cursor,
	)
	return err
}

func recordResult(resource string, err error) {
	result := "success"
	if err != nil {
		result = "error"
	}
	upsertsTotal.WithLabelValues(resource, result).Inc()
}
