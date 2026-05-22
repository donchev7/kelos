package slack

import (
	"fmt"
	"strings"

	goslack "github.com/slack-go/slack"
)

const selfHandoffStopNoticePrefix = "Auto-handoff stopped after "

type selfHandoffCommand struct {
	Raw        string
	Normalized string
	Target     string
}

func extractSelfHandoffCommands(text, botUserID string) []selfHandoffCommand {
	var commands []selfHandoffCommand
	inFence := false

	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			continue
		}
		if inFence || trimmed == "" {
			continue
		}
		if cmd, ok := parseSelfHandoffCommandLine(trimmed, botUserID); ok {
			commands = append(commands, cmd)
		}
	}

	return commands
}

func parseSelfHandoffCommandLine(line, botUserID string) (selfHandoffCommand, bool) {
	rest, ok := cutLiteralCodyMention(line)
	if !ok {
		rest, ok = cutSlackBotMention(line, botUserID)
	}
	if !ok {
		return selfHandoffCommand{}, false
	}

	rest = strings.TrimSpace(rest)
	if rest == "" {
		return selfHandoffCommand{}, false
	}

	target, afterTarget, ok := cutAllowedSelfHandoffTarget(rest)
	if !ok {
		return selfHandoffCommand{}, false
	}

	normalized := strings.TrimSpace(fmt.Sprintf("<@%s> !%s %s", botUserID, target, strings.TrimSpace(afterTarget)))
	return selfHandoffCommand{
		Raw:        line,
		Normalized: normalized,
		Target:     target,
	}, true
}

func cutLiteralCodyMention(line string) (string, bool) {
	if line == "@cody" {
		return "", true
	}
	if strings.HasPrefix(line, "@cody ") || strings.HasPrefix(line, "@cody\t") {
		return line[len("@cody"):], true
	}
	return "", false
}

func cutSlackBotMention(line, botUserID string) (string, bool) {
	if botUserID == "" || !strings.HasPrefix(line, "<@") {
		return "", false
	}
	end := strings.Index(line, ">")
	if end == -1 {
		return "", false
	}

	mention := line[:end+1]
	if mention != fmt.Sprintf("<@%s>", botUserID) && !strings.HasPrefix(mention, fmt.Sprintf("<@%s|", botUserID)) {
		return "", false
	}

	rest := line[end+1:]
	if rest != "" && !strings.HasPrefix(rest, " ") && !strings.HasPrefix(rest, "\t") {
		return "", false
	}
	return rest, true
}

func cutAllowedSelfHandoffTarget(rest string) (target, after string, ok bool) {
	for _, candidate := range []string{"ticket", "dev", "review", "babysit"} {
		prefix := "!" + candidate
		if rest == prefix {
			return candidate, "", true
		}
		if strings.HasPrefix(rest, prefix+" ") || strings.HasPrefix(rest, prefix+"\t") {
			return candidate, rest[len(prefix):], true
		}
	}
	return "", "", false
}

func countPriorSelfHandoffCommands(messages []goslack.Message, currentTS, botUserID, botID string) (count int, mostRecentNormalized string) {
	for _, msg := range messages {
		if currentTS != "" && !slackTimestampBefore(msg.Timestamp, currentTS) {
			continue
		}
		if !isSelfAuthoredSlackMessage(msg.User, msg.BotID, botUserID, botID) {
			continue
		}
		commands := extractSelfHandoffCommands(msg.Text, botUserID)
		count += len(commands)
		if len(commands) > 0 {
			mostRecentNormalized = commands[len(commands)-1].Normalized
		}
	}
	return count, mostRecentNormalized
}

func slackTimestampBefore(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return a < b
}

func isSelfAuthoredSlackMessage(userID, botID, selfUserID, selfBotID string) bool {
	if selfUserID != "" && userID == selfUserID {
		return true
	}
	return selfBotID != "" && botID == selfBotID
}

func threadHasSelfHandoffStopNotice(messages []goslack.Message, botUserID, botID string) bool {
	for _, msg := range messages {
		if !isSelfAuthoredSlackMessage(msg.User, msg.BotID, botUserID, botID) {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(msg.Text), selfHandoffStopNoticePrefix) {
			return true
		}
	}
	return false
}
