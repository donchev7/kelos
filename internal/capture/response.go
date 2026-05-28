package capture

import (
	"bufio"
	"os"
	"strings"
)

type responseAccumulator interface {
	addLine(line []byte)
	result() string
}

func newResponseAccumulator(agentType string) responseAccumulator {
	switch agentType {
	case "claude-code":
		return &assistantResponseAccumulator{join: "\n\n", preferResult: true}
	case "codex":
		return &codexResponseAccumulator{}
	case "gemini":
		return &assistantResponseAccumulator{join: "", includeTextEvents: true}
	case "opencode":
		return &opencodeResponseAccumulator{}
	case "cursor":
		return &assistantResponseAccumulator{join: "\n\n", preferResult: true}
	default:
		return nil
	}
}

// ParseResponse extracts the agent's final response text from an agent output
// file. It is kept for tests and callers that still operate on captured files;
// the normal runtime path extracts the response while streaming.
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

type assistantResponseAccumulator struct {
	parts             []string
	finalResult       string
	join              string
	preferResult      bool
	includeTextEvents bool
}

func (a *assistantResponseAccumulator) addLine(line []byte) {
	m := parseLine(line)
	if m == nil {
		return
	}
	if a.preferResult && m["type"] == "result" {
		if result, ok := m["result"].(string); ok && result != "" {
			a.finalResult = result
		}
		return
	}
	if a.includeTextEvents && m["type"] == "text" {
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

func (a *assistantResponseAccumulator) result() string {
	if a.preferResult && a.finalResult != "" {
		return a.finalResult
	}
	return strings.Join(a.parts, a.join)
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
		if part, ok := m["part"].(map[string]any); ok {
			if text, ok := part["text"].(string); ok && text != "" {
				a.parts = append(a.parts, text)
			}
		}
	case "step_finish":
		if part, ok := m["part"].(map[string]any); ok {
			if text, ok := part["text"].(string); ok && text != "" {
				a.parts = append(a.parts, text)
			}
		}
	}
}

func (a *opencodeResponseAccumulator) result() string {
	return strings.Join(a.parts, "")
}
