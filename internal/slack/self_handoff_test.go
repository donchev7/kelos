package slack

import (
	"testing"

	goslack "github.com/slack-go/slack"
)

func TestExtractSelfHandoffCommands(t *testing.T) {
	tests := []struct {
		name           string
		text           string
		wantNormalized []string
		wantTargets    []string
	}{
		{
			name: "literal cody dev command",
			text: "Done.\n\nNext step:\n@cody !dev please implement the ticket above.",
			wantNormalized: []string{
				"<@UBOT> !dev please implement the ticket above.",
			},
			wantTargets: []string{"dev"},
		},
		{
			name: "slack mention review command",
			text: "<@UBOT> !review please review the PR above.",
			wantNormalized: []string{
				"<@UBOT> !review please review the PR above.",
			},
			wantTargets: []string{"review"},
		},
		{
			name: "slack mention with display name",
			text: "<@UBOT|cody> !babysit https://github.com/o/r/pull/1",
			wantNormalized: []string{
				"<@UBOT> !babysit https://github.com/o/r/pull/1",
			},
			wantTargets: []string{"babysit"},
		},
		{
			name: "command in code fence ignored",
			text: "Example:\n```\n@cody !dev do not run this\n```\nDone.",
		},
		{
			name: "inline command ignored",
			text: "Next step: @cody !dev do not run inline commands",
		},
		{
			name: "unsupported target ignored",
			text: "@cody !debug diagnose this",
		},
		{
			name: "multiple command lines returned",
			text: "@cody !dev fix it\n@cody !review review it",
			wantNormalized: []string{
				"<@UBOT> !dev fix it",
				"<@UBOT> !review review it",
			},
			wantTargets: []string{"dev", "review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSelfHandoffCommands(tt.text, "UBOT")
			if len(got) != len(tt.wantNormalized) {
				t.Fatalf("got %d commands, want %d: %#v", len(got), len(tt.wantNormalized), got)
			}
			for i := range got {
				if got[i].Normalized != tt.wantNormalized[i] {
					t.Errorf("command[%d].Normalized = %q, want %q", i, got[i].Normalized, tt.wantNormalized[i])
				}
				if got[i].Target != tt.wantTargets[i] {
					t.Errorf("command[%d].Target = %q, want %q", i, got[i].Target, tt.wantTargets[i])
				}
			}
		})
	}
}

func TestCountPriorSelfHandoffCommands(t *testing.T) {
	messages := []goslack.Message{
		{Msg: goslack.Msg{User: "UBOT", Timestamp: "1000.000001", Text: "@cody !dev fix the ticket"}},
		{Msg: goslack.Msg{User: "U1", Timestamp: "1000.000002", Text: "@cody !review human command"}},
		{Msg: goslack.Msg{BotID: "BCODY", Timestamp: "1000.000003", Text: "@cody !review review the PR"}},
		{Msg: goslack.Msg{BotID: "BCODY", Timestamp: "1000.000004", Text: "@cody !dev current command"}},
	}

	count, recent := countPriorSelfHandoffCommands(messages, "1000.000004", "UBOT", "BCODY")
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if recent != "<@UBOT> !review review the PR" {
		t.Fatalf("recent = %q, want review command", recent)
	}
}

func TestThreadHasSelfHandoffStopNotice(t *testing.T) {
	messages := []goslack.Message{
		{Msg: goslack.Msg{BotID: "BCODY", Text: "Auto-handoff stopped after 4 Cody handoffs in this thread. Please continue manually if needed."}},
	}

	if !threadHasSelfHandoffStopNotice(messages, "UBOT", "BCODY") {
		t.Fatal("expected stop notice to be detected")
	}
}
