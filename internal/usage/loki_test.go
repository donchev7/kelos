package usage

import (
	"testing"
	"time"
)

func TestParseLokiAgentEntriesMergesSlackTaskMetadata(t *testing.T) {
	base := time.Date(2026, 5, 23, 13, 18, 29, 0, time.UTC)
	agent := []lokiEntry{
		{
			Timestamp: base.Add(time.Second),
			Labels: map[string]string{
				"namespace":    "kelos-system",
				"container":    "codex",
				"app":          "cody-debug-slack-ce79e7c7b9c9",
				"service_name": "cody-debug-slack-ce79e7c7b9c9",
				"pod":          "cody-debug-slack-ce79e7c7b9c9-fx8cz",
			},
			Line: "---KELOS_OUTPUTS_START---",
		},
		{
			Timestamp: base.Add(2 * time.Second),
			Labels: map[string]string{
				"namespace":    "kelos-system",
				"container":    "codex",
				"app":          "cody-debug-slack-ce79e7c7b9c9",
				"service_name": "cody-debug-slack-ce79e7c7b9c9",
				"pod":          "cody-debug-slack-ce79e7c7b9c9-fx8cz",
			},
			Line: "input-tokens: 64247",
		},
		{
			Timestamp: base.Add(3 * time.Second),
			Labels: map[string]string{
				"namespace":    "kelos-system",
				"container":    "codex",
				"app":          "cody-debug-slack-ce79e7c7b9c9",
				"service_name": "cody-debug-slack-ce79e7c7b9c9",
				"pod":          "cody-debug-slack-ce79e7c7b9c9-fx8cz",
			},
			Line: "output-tokens: 282",
		},
		{
			Timestamp: base.Add(4 * time.Second),
			Labels: map[string]string{
				"namespace":    "kelos-system",
				"container":    "codex",
				"app":          "cody-debug-slack-ce79e7c7b9c9",
				"service_name": "cody-debug-slack-ce79e7c7b9c9",
				"pod":          "cody-debug-slack-ce79e7c7b9c9-fx8cz",
			},
			Line: "---KELOS_OUTPUTS_END---",
		},
	}
	slack := []lokiEntry{
		{
			Timestamp: base,
			Labels:    map[string]string{"namespace": "kelos-system", "pod": "kelos-slack-server-abc"},
			Line:      `{"msg":"Message matches TaskSpawner — creating task","spawner":"cody-debug","namespace":"kelos-system","channel":"C0B477EMD8W","user":"U05AYPP2C48"}`,
		},
		{
			Timestamp: base.Add(500 * time.Millisecond),
			Labels:    map[string]string{"namespace": "kelos-system", "pod": "kelos-slack-server-abc"},
			Line:      `{"msg":"Created task from Slack message","task":"cody-debug-slack-ce79e7c7b9c9","spawner":"cody-debug"}`,
		},
	}

	bundles := parseLokiAgentEntries(agent, "non-prod", "kelos-system")
	mergeSlackEntries(bundles, slack, "non-prod", "kelos-system")

	bundle := bundles[LokiTaskTurnID("kelos-system", "cody-debug-slack-ce79e7c7b9c9")]
	if bundle == nil {
		t.Fatal("expected task bundle")
	}
	if bundle.Turn.TaskSpawnerName != "cody-debug" {
		t.Errorf("spawner = %q", bundle.Turn.TaskSpawnerName)
	}
	if got := ptrStringValue(bundle.Turn.SlackUserID); got != "U05AYPP2C48" {
		t.Errorf("slack user = %q", got)
	}
	if got := ptrStringValue(bundle.Turn.SlackChannelID); got != "C0B477EMD8W" {
		t.Errorf("slack channel = %q", got)
	}
	if bundle.Turn.InputTokens == nil || *bundle.Turn.InputTokens != 64247 {
		t.Errorf("input tokens = %v", bundle.Turn.InputTokens)
	}
	if bundle.Turn.OutputTokens == nil || *bundle.Turn.OutputTokens != 282 {
		t.Errorf("output tokens = %v", bundle.Turn.OutputTokens)
	}
	if bundle.Turn.TotalTokens == nil || *bundle.Turn.TotalTokens != 64529 {
		t.Errorf("total tokens = %v", bundle.Turn.TotalTokens)
	}
}

func TestMergeSlackEntriesCreatesSessionTurnRows(t *testing.T) {
	base := time.Date(2026, 5, 23, 14, 0, 0, 0, time.UTC)
	slack := []lokiEntry{
		{
			Timestamp: base,
			Labels:    map[string]string{"namespace": "kelos-system", "pod": "kelos-slack-server-abc"},
			Line:      `{"msg":"Message matches session-enabled TaskSpawner — creating AgentSession turn","spawner":"cody-dev","namespace":"kelos-system","channel":"C123","user":"U123"}`,
		},
		{
			Timestamp: base.Add(time.Second),
			Labels:    map[string]string{"namespace": "kelos-system", "pod": "kelos-slack-server-abc"},
			Line:      `{"msg":"Posting Slack accepted reply for AgentTurn","turn":"cody-dev-sess-abc123-t-0001","channel":"C123"}`,
		},
		{
			Timestamp: base.Add(2 * time.Second),
			Labels:    map[string]string{"namespace": "kelos-system", "pod": "kelos-slack-server-abc"},
			Line:      `{"msg":"Posting Slack terminal reply for AgentTurn","turn":"cody-dev-sess-abc123-t-0001","channel":"C123"}`,
		},
	}

	bundles := map[string]*lokiBundle{}
	mergeSlackEntries(bundles, slack, "non-prod", "kelos-system")

	bundle := bundles[LokiTurnID("kelos-system", "cody-dev-sess-abc123-t-0001")]
	if bundle == nil {
		t.Fatal("expected AgentTurn bundle")
	}
	if bundle.Turn.TaskSpawnerName != "cody-dev" {
		t.Errorf("spawner = %q", bundle.Turn.TaskSpawnerName)
	}
	if got := ptrStringValue(bundle.Turn.SlackUserID); got != "U123" {
		t.Errorf("slack user = %q", got)
	}
	if bundle.Turn.CompletedAt == nil {
		t.Fatal("expected terminal event to set completed_at")
	}
	if bundle.Turn.Phase != "Succeeded" {
		t.Errorf("phase = %q", bundle.Turn.Phase)
	}
}
