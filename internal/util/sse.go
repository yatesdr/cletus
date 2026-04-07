package util

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"regexp"
)

// SSEParser parses Server-Sent Events
type SSEParser struct {
	dataBuffer bytes.Buffer
}

// NewSSEParser creates a new SSE parser
func NewSSEParser() *SSEParser {
	return &SSEParser{}
}

// ParseEvent parses a single SSE event line
func ParseEvent(line string) (eventType string, data string, ok bool) {
	// Remove leading/trailing whitespace
	line = trimSpaces(line)
	if len(line) == 0 {
		return "", "", false
	}

	// Skip comments
	if line[0] == ':' {
		return "", "", false
	}

	// Parse field
	colonIdx := bytes.IndexByte([]byte(line), ':')
	if colonIdx == -1 {
		// No colon - treat entire line as event type
		return line, "", true
	}

	field := line[:colonIdx]
	value := line[colonIdx+1:]

	// Skip leading space on value
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}

	switch field {
	case "event":
		return value, "", true
	case "data":
		return "", value, true
	default:
		return "", "", false
	}
}

// SSEEvent represents a parsed SSE event
type SSEEvent struct {
	Type string
	Data string
}

// Reader creates an SSE event reader from an io.Reader
func Reader(r io.Reader) <-chan SSEEvent {
	ch := make(chan SSEEvent, 10)
	go func() {
		defer close(ch)
		reader := bufio.NewReader(r)
		
		var currentType string
		var dataBuf bytes.Buffer

		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if dataBuf.Len() > 0 {
					ch <- SSEEvent{Type: currentType, Data: dataBuf.String()}
				}
				if err != io.EOF {
					ch <- SSEEvent{Type: "error", Data: err.Error()}
				}
				return
			}

			// Trim line ending
			line = bytes.TrimSuffix(line, []byte("\r\n"))
			line = bytes.TrimSuffix(line, []byte("\n"))
			line = bytes.TrimSuffix(line, []byte("\r"))

			// Empty line = event end
			if len(line) == 0 {
				if dataBuf.Len() > 0 {
					ch <- SSEEvent{Type: currentType, Data: dataBuf.String()}
					dataBuf.Reset()
				}
				currentType = ""
				continue
			}

			eventType, data, ok := ParseEvent(string(line))
			if !ok {
				continue
			}

			if eventType != "" {
				currentType = eventType
			}
			if data != "" {
				dataBuf.WriteString(data)
				dataBuf.WriteByte('\n')
			}
		}
	}()

	return ch
}

// ParseJSON parses JSON from SSE data
func ParseJSON[T any](data string) (T, error) {
	var result T
	// Handle multiple JSON objects (some SSE responses have multiple data lines)
	// Just parse the first complete JSON
	err := json.Unmarshal([]byte(data), &result)
	return result, err
}

// IsUUID checks if a string is a UUID
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func IsUUID(s string) bool {
	return uuidRegex.MatchString(s)
}

func trimSpaces(s string) string {
	start := 0
	end := len(s)
	for start < end && s[start] == ' ' {
		start++
	}
	for end > start && s[end-1] == ' ' {
		end--
	}
	return s[start:end]
}
