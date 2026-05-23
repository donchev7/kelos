CREATE TABLE IF NOT EXISTS cody_usage_sessions (
  session_id TEXT PRIMARY KEY,
  cluster TEXT NOT NULL,
  namespace TEXT NOT NULL,
  taskspawner_name TEXT NOT NULL,
  persona TEXT NOT NULL DEFAULT 'unknown',
  source TEXT NOT NULL DEFAULT 'slack',
  slack_team_id TEXT,
  slack_channel_id TEXT,
  slack_root_ts TEXT,
  first_user_id TEXT,
  first_seen_at TIMESTAMPTZ NOT NULL,
  last_activity_at TIMESTAMPTZ NOT NULL,
  first_task_name TEXT,
  agent_session_name TEXT,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS cody_usage_sessions_seen_idx
  ON cody_usage_sessions (first_seen_at);

CREATE INDEX IF NOT EXISTS cody_usage_sessions_taskspawner_idx
  ON cody_usage_sessions (taskspawner_name, first_seen_at);

CREATE INDEX IF NOT EXISTS cody_usage_sessions_user_idx
  ON cody_usage_sessions (first_user_id, first_seen_at);

CREATE TABLE IF NOT EXISTS cody_usage_turns (
  turn_id TEXT PRIMARY KEY,
  session_id TEXT NOT NULL REFERENCES cody_usage_sessions(session_id),
  cluster TEXT NOT NULL,
  namespace TEXT NOT NULL,
  taskspawner_name TEXT NOT NULL,
  persona TEXT NOT NULL DEFAULT 'unknown',
  source TEXT NOT NULL DEFAULT 'slack',
  slack_user_id TEXT,
  slack_channel_id TEXT,
  slack_thread_ts TEXT,
  slack_message_ts TEXT,
  task_name TEXT,
  task_uid TEXT,
  agent_turn_name TEXT,
  agent_turn_uid TEXT,
  agent_type TEXT,
  model TEXT,
  phase TEXT NOT NULL DEFAULT 'Pending',
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  duration_seconds DOUBLE PRECISION,
  input_tokens BIGINT,
  output_tokens BIGINT,
  total_tokens BIGINT,
  cost_usd NUMERIC(18, 8),
  pr_url TEXT,
  error_message TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS cody_usage_turns_completed_idx
  ON cody_usage_turns (completed_at);

CREATE INDEX IF NOT EXISTS cody_usage_turns_started_idx
  ON cody_usage_turns (started_at);

CREATE INDEX IF NOT EXISTS cody_usage_turns_user_idx
  ON cody_usage_turns (slack_user_id, started_at);

CREATE INDEX IF NOT EXISTS cody_usage_turns_taskspawner_idx
  ON cody_usage_turns (taskspawner_name, started_at);

CREATE INDEX IF NOT EXISTS cody_usage_turns_phase_idx
  ON cody_usage_turns (phase, started_at);

CREATE TABLE IF NOT EXISTS cody_usage_collector_offsets (
  source TEXT PRIMARY KEY,
  cursor TEXT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE OR REPLACE VIEW cody_usage_daily AS
SELECT
  date_trunc('day', COALESCE(started_at, created_at)) AS day,
  taskspawner_name,
  persona,
  count(*) AS turns,
  count(DISTINCT session_id) AS sessions,
  count(DISTINCT slack_user_id) FILTER (WHERE slack_user_id IS NOT NULL) AS users,
  count(*) FILTER (WHERE phase = 'Succeeded') AS succeeded,
  count(*) FILTER (WHERE phase = 'Failed') AS failed,
  avg(duration_seconds) FILTER (WHERE duration_seconds IS NOT NULL) AS avg_duration_seconds,
  percentile_disc(0.95) WITHIN GROUP (ORDER BY duration_seconds)
    FILTER (WHERE duration_seconds IS NOT NULL) AS p95_duration_seconds,
  sum(input_tokens) AS input_tokens,
  sum(output_tokens) AS output_tokens,
  sum(cost_usd) AS cost_usd
FROM cody_usage_turns
GROUP BY 1, 2, 3;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'cody_usage_readonly') THEN
    GRANT USAGE ON SCHEMA public TO cody_usage_readonly;
    GRANT SELECT ON ALL TABLES IN SCHEMA public TO cody_usage_readonly;
    GRANT SELECT ON ALL SEQUENCES IN SCHEMA public TO cody_usage_readonly;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public
      GRANT SELECT ON TABLES TO cody_usage_readonly;
  END IF;
END $$;
