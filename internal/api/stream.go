package api

import (
	"encoding/json"
	"fmt"
	"cletus/internal/util"
	"io"
)

// StreamHandler processes streaming API responses
type StreamHandler struct {
	ContentBlocks   []ContentBlock
	currentBlock    *ContentBlock
	textBuffer      string
	thinkingBuffer  string
	inputJSONBuffer string
	Usage           Usage
	Done            bool
	Error           error
}

// NewStreamHandler creates a new stream handler
func NewStreamHandler() *StreamHandler {
	return &StreamHandler{
		ContentBlocks: make([]ContentBlock, 0),
	}
}

// HandleEvent processes a single SSE event
func (sh *StreamHandler) HandleEvent(event util.SSEEvent) {
	switch event.Type {
	case "message_start":
		var msg struct {
			Type string `json:"type"`
			Usage Usage `json:"usage"`
		}
		if json.Unmarshal([]byte(event.Data), &msg) == nil {
			sh.Usage = msg.Usage
		}

	case "content_block_start":
		var blockStart ContentBlockStart
		if err := json.Unmarshal([]byte(event.Data), &blockStart); err == nil {
			sh.currentBlock = &blockStart.ContentBlock
			sh.ContentBlocks = append(sh.ContentBlocks, *sh.currentBlock)
		}

	case "content_block_delta":
		if sh.currentBlock == nil {
			return
		}
		
		var delta struct {
			Type        string `json:"type"`
			Text        string `json:"text,omitempty"`
			PartialJSON string `json:"partial_json,omitempty"`
			Thinking   string `json:"thinking,omitempty"`
		}
		if err := json.Unmarshal([]byte(event.Data), &delta); err != nil {
			return
		}

		switch delta.Type {
		case "text_delta":
			sh.textBuffer += delta.Text
			sh.currentBlock.Text = sh.textBuffer
		case "input_json_delta":
			sh.inputJSONBuffer += delta.PartialJSON
			sh.currentBlock.Input = json.RawMessage(sh.inputJSONBuffer)
		case "thinking_delta":
			sh.thinkingBuffer += delta.Thinking
		}

	case "content_block_stop":
		sh.currentBlock = nil
		sh.textBuffer = ""
		sh.thinkingBuffer = ""
		sh.inputJSONBuffer = ""

	case "message_delta":
		var delta struct {
			Type string `json:"type"`
			Usage Usage `json:"usage"`
		}
		if json.Unmarshal([]byte(event.Data), &delta) == nil {
			sh.Usage = delta.Usage
		}

	case "message_stop":
		sh.Done = true

	case "error":
		var apiErr APIError
		if json.Unmarshal([]byte(event.Data), &apiErr) == nil {
			errMsg := apiErr.Message
			sh.Error = fmt.Errorf("API error: %s", errMsg)
		}
	}
}

// ReadStream reads and processes a streaming response
func ReadStream(body io.Reader) (*StreamHandler, error) {
	handler := NewStreamHandler()
	
	events := util.Reader(body)
	for event := range events {
		handler.HandleEvent(event)
		if handler.Error != nil || handler.Done {
			break
		}
	}
	
	return handler, nil
}

// GetToolUses extracts tool_use blocks from the content
func (sh *StreamHandler) GetToolUses() []ContentBlock {
	var tools []ContentBlock
	for _, block := range sh.ContentBlocks {
		if block.Type == "tool_use" {
			tools = append(tools, block)
		}
	}
	return tools
}

// GetTextContent returns all text content blocks joined
func (sh *StreamHandler) GetTextContent() string {
	var text string
	for _, block := range sh.ContentBlocks {
		if block.Type == "text" {
			text += block.Text
		}
	}
	return text
}
