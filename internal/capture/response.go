package capture

import (
	"bufio"
	"os"
	"strings"
)

// ParseResponse extracts the agent's final response text from the agent
// output file. The returned string is the user-visible answer (or
// concatenation of answers across turns) and is intended for reporters
// that surface task results back to the originating channel (Slack thread,
// GitHub PR comment, etc.).
//
// Returns an empty string if the file is unreadable, the agent type is
// unknown, or the agent produced no extractable response text.
func ParseResponse(agentType, filePath string) string {
	acc := newResponseAccumulator(agentType)
	if acc == nil {
		return ""
	}
	f, err := os.Open(filePath)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		acc.addLine(scanner.Bytes())
	}
	return acc.result()
}

type responseAccumulator interface {
	addLine(line []byte)
	result() string
}

func newResponseAccumulator(agentType string) responseAccumulator {
	switch agentType {
	case "claude-code":
		return &assistantResultResponseAccumulator{joiner: "\n\n"}
	case "codex":
		return &codexResponseAccumulator{}
	case "gemini":
		return &geminiResponseAccumulator{}
	case "opencode":
		return &opencodeResponseAccumulator{}
	case "cursor":
		return &assistantResultResponseAccumulator{joiner: "\n\n"}
	default:
		return nil
	}
}

type codexResponseAccumulator struct {
	parts []string
}

func (a *codexResponseAccumulator) addLine(line []byte) {
	m := parseLine(line)
	if m == nil || m["type"] != "item.completed" {
		return
	}
	item, ok := m["item"].(map[string]any)
	if !ok || item["type"] != "agent_message" {
		return
	}
	if text, ok := item["text"].(string); ok && text != "" {
		a.parts = append(a.parts, text)
	}
}

func (a *codexResponseAccumulator) result() string {
	return strings.Join(a.parts, "\n\n")
}

// assistantResultResponseAccumulator handles agent formats where assistant
// messages contain text blocks and an optional trailing result carries the
// preferred final answer.
type assistantResultResponseAccumulator struct {
	final  string
	parts  []string
	joiner string
}

func (a *assistantResultResponseAccumulator) addLine(line []byte) {
	m := parseLine(line)
	if m == nil {
		return
	}
	if m["type"] == "result" {
		if result, ok := m["result"].(string); ok && result != "" {
			a.final = result
		}
		return
	}
	if m["type"] != "assistant" {
		return
	}
	a.addAssistantContent(m)
}

func (a *assistantResultResponseAccumulator) addAssistantContent(m map[string]any) {
	msg, ok := m["message"].(map[string]any)
	if !ok {
		return
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return
	}
	for _, block := range content {
		b, ok := block.(map[string]any)
		if !ok || b["type"] != "text" {
			continue
		}
		if text, ok := b["text"].(string); ok && text != "" {
			a.parts = append(a.parts, text)
		}
	}
}

func (a *assistantResultResponseAccumulator) result() string {
	if a.final != "" {
		return a.final
	}
	return strings.Join(a.parts, a.joiner)
}

type geminiResponseAccumulator struct {
	parts []string
}

func (a *geminiResponseAccumulator) addLine(line []byte) {
	m := parseLine(line)
	if m == nil {
		return
	}
	if m["type"] == "text" {
		if text, ok := m["text"].(string); ok && text != "" {
			a.parts = append(a.parts, text)
		}
		return
	}
	if m["type"] != "assistant" {
		return
	}
	msg, ok := m["message"].(map[string]any)
	if !ok {
		return
	}
	content, ok := msg["content"].([]any)
	if !ok {
		return
	}
	for _, block := range content {
		b, ok := block.(map[string]any)
		if !ok || b["type"] != "text" {
			continue
		}
		if text, ok := b["text"].(string); ok && text != "" {
			a.parts = append(a.parts, text)
		}
	}
}

func (a *geminiResponseAccumulator) result() string {
	return strings.Join(a.parts, "")
}

type opencodeResponseAccumulator struct {
	parts []string
}

func (a *opencodeResponseAccumulator) addLine(line []byte) {
	m := parseLine(line)
	if m == nil {
		return
	}
	switch m["type"] {
	case "text":
		if text, ok := m["text"].(string); ok && text != "" {
			a.parts = append(a.parts, text)
			return
		}
		a.addPartText(m)
	case "step_finish":
		a.addPartText(m)
	}
}

func (a *opencodeResponseAccumulator) addPartText(m map[string]any) {
	part, ok := m["part"].(map[string]any)
	if !ok {
		return
	}
	if text, ok := part["text"].(string); ok && text != "" {
		a.parts = append(a.parts, text)
	}
}

func (a *opencodeResponseAccumulator) result() string {
	return strings.Join(a.parts, "")
}
