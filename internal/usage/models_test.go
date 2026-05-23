package usage

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	kelosv1alpha1 "github.com/kelos-dev/kelos/api/v1alpha1"
	"github.com/kelos-dev/kelos/internal/reporting"
)

func TestRecordsFromTask(t *testing.T) {
	start := metav1.NewTime(time.Date(2026, 5, 23, 10, 0, 0, 0, time.UTC))
	done := metav1.NewTime(time.Date(2026, 5, 23, 10, 2, 0, 0, time.UTC))
	task := &kelosv1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cody-debug-slack-abc123",
			Namespace:         "kelos-system",
			UID:               types.UID("task-uid"),
			CreationTimestamp: metav1.NewTime(start.Add(-time.Minute)),
			Labels: map[string]string{
				labelTaskSpawner: "cody-debug",
			},
			Annotations: map[string]string{
				reporting.AnnotationSlackChannel:  "C123",
				reporting.AnnotationSlackThreadTS: "1716460000.000100",
				reporting.AnnotationSlackUserID:   "U123",
			},
		},
		Spec: kelosv1alpha1.TaskSpec{
			Type:  "codex",
			Model: "gpt-5",
		},
		Status: kelosv1alpha1.TaskStatus{
			Phase:          kelosv1alpha1.TaskPhaseSucceeded,
			StartTime:      &start,
			CompletionTime: &done,
			Results: map[string]string{
				"input-tokens":  "100",
				"output_tokens": "50",
				"costUSD":       "0.0123",
				"pr":            "https://github.com/org/repo/pull/1",
			},
		},
	}

	session, turn := RecordsFromTask(task, "non-prod", nil)

	if session.SessionID == "" || session.SessionID[:6] != "slack-" {
		t.Fatalf("session id = %q, want slack-derived id", session.SessionID)
	}
	if session.TaskSpawnerName != "cody-debug" {
		t.Errorf("session spawner = %q", session.TaskSpawnerName)
	}
	if session.Persona != "debug" {
		t.Errorf("persona = %q, want debug", session.Persona)
	}
	if turn.TurnID != "task-task-uid" {
		t.Errorf("turn id = %q", turn.TurnID)
	}
	if got := ptrStringValue(turn.SlackUserID); got != "U123" {
		t.Errorf("slack user = %q", got)
	}
	if turn.InputTokens == nil || *turn.InputTokens != 100 {
		t.Errorf("input tokens = %v", turn.InputTokens)
	}
	if turn.OutputTokens == nil || *turn.OutputTokens != 50 {
		t.Errorf("output tokens = %v", turn.OutputTokens)
	}
	if turn.TotalTokens == nil || *turn.TotalTokens != 150 {
		t.Errorf("total tokens = %v", turn.TotalTokens)
	}
	if got := ptrStringValue(turn.CostUSD); got != "0.0123" {
		t.Errorf("cost = %q", got)
	}
	if turn.DurationSeconds == nil || *turn.DurationSeconds != 120 {
		t.Errorf("duration = %v", turn.DurationSeconds)
	}
}

func TestRecordsFromAgentTurn(t *testing.T) {
	start := metav1.NewTime(time.Date(2026, 5, 23, 11, 0, 0, 0, time.UTC))
	done := metav1.NewTime(time.Date(2026, 5, 23, 11, 1, 0, 0, time.UTC))
	session := &kelosv1alpha1.AgentSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cody-dev-sess-abc123",
			Namespace: "kelos-system",
			UID:       types.UID("session-uid"),
		},
		Spec: kelosv1alpha1.AgentSessionSpec{
			TaskSpawnerRef: kelosv1alpha1.TaskSpawnerReference{Name: "cody-dev"},
			Source: kelosv1alpha1.AgentSessionSource{
				Type:      "SlackThread",
				ChannelID: "C123",
				RootTS:    "1716460000.000100",
			},
		},
		Status: kelosv1alpha1.AgentSessionStatus{Phase: kelosv1alpha1.AgentSessionPhaseRunning},
	}
	turn := &kelosv1alpha1.AgentTurn{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cody-dev-sess-abc123-t-0001",
			Namespace:         "kelos-system",
			UID:               types.UID("turn-uid"),
			CreationTimestamp: start,
			Labels:            map[string]string{labelTaskSpawner: "cody-dev"},
		},
		Spec: kelosv1alpha1.AgentTurnSpec{
			SessionRef: kelosv1alpha1.AgentSessionReference{Name: session.Name},
			Source: kelosv1alpha1.AgentTurnSource{
				Type:      "SlackMessage",
				ChannelID: "C123",
				RootTS:    "1716460000.000100",
				MessageTS: "1716460001.000200",
				UserID:    "U123",
			},
		},
		Status: kelosv1alpha1.AgentTurnStatus{
			Phase:       kelosv1alpha1.AgentTurnPhaseSucceeded,
			StartedAt:   &start,
			CompletedAt: &done,
		},
	}

	sessionRecord, turnRecord := RecordsFromAgentTurn(turn, session, "non-prod", nil)

	if sessionRecord.AgentSessionName == nil || *sessionRecord.AgentSessionName != session.Name {
		t.Errorf("agent session name = %v", sessionRecord.AgentSessionName)
	}
	if turnRecord.TurnID != "agentturn-turn-uid" {
		t.Errorf("turn id = %q", turnRecord.TurnID)
	}
	if turnRecord.TaskSpawnerName != "cody-dev" {
		t.Errorf("spawner = %q", turnRecord.TaskSpawnerName)
	}
	if turnRecord.Persona != "dev" {
		t.Errorf("persona = %q", turnRecord.Persona)
	}
	if got := ptrStringValue(turnRecord.SlackMessageTS); got != "1716460001.000200" {
		t.Errorf("message ts = %q", got)
	}
}

func TestPersonaForPrefersLabels(t *testing.T) {
	if got := personaFor("cody-debug", map[string]string{labelAlpheyaPersona: "pr-reviewer"}); got != "pr-reviewer" {
		t.Errorf("persona = %q, want Alpheya label value", got)
	}
	if got := personaFor("cody-debug", map[string]string{labelPersona: "debug-custom", labelAlpheyaPersona: "pr-reviewer"}); got != "debug-custom" {
		t.Errorf("persona = %q, want Kelos label value", got)
	}
}
