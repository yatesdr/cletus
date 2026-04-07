package util

import (
	"strings"
	"testing"
)

func TestSSEReader(t *testing.T) {
	input := "event: message_start\ndata: {\"type\":\"message_start\"}\n\nevent: content_block_delta\ndata: {\"type\":\"text_delta\",\"text\":\"Hello\"}\n\nevent: message_stop\ndata: {}\n\n"

	events := Reader(strings.NewReader(input))

	var collected []SSEEvent
	for e := range events {
		collected = append(collected, e)
	}

	if len(collected) != 3 {
		t.Fatalf("expected 3 events, got %d", len(collected))
	}
	if collected[0].Type != "message_start" {
		t.Errorf("event 0: expected message_start, got %s", collected[0].Type)
	}
	if collected[1].Type != "content_block_delta" {
		t.Errorf("event 1: expected content_block_delta, got %s", collected[1].Type)
	}
	if collected[2].Type != "message_stop" {
		t.Errorf("event 2: expected message_stop, got %s", collected[2].Type)
	}
}

func TestSSEReaderMultiLine(t *testing.T) {
	input := "event: message_start\ndata: {\"type\":\"message_start\",\"usage\":{\"input_tokens\":100}}\n\nevent: content_block_delta\ndata: {\"type\":\"text_delta\",\"text\":\"Line1\nLine2\"}\n\nevent: message_stop\ndata: {}\n\n"

	events := Reader(strings.NewReader(input))

	var collected []SSEEvent
	for e := range events {
		collected = append(collected, e)
	}

	if len(collected) != 3 {
		t.Fatalf("expected 3 events, got %d", len(collected))
	}
	if !strings.Contains(collected[1].Data, "Line1") {
		t.Errorf("event 1 data should contain Line1, got: %s", collected[1].Data)
	}
}
