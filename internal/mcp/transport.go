package mcp

import (
	"context"
	"fmt"
	"io"
	"os/exec"
)

// Transport represents a transport mechanism for MCP
type Transport interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Send(ctx context.Context, msg []byte) ([]byte, error)
	IsConnected() bool
}

// StdioTransport uses stdio for communication
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(command string, args ...string) *StdioTransport {
	return &StdioTransport{
		cmd: exec.Command(command, args...),
	}
}

// Connect starts the process
func (t *StdioTransport) Connect(ctx context.Context) error {
	t.cmd.StdinPipe()
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	t.stdout = stdout

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	return nil
}

// Disconnect stops the process
func (t *StdioTransport) Disconnect() error {
	if t.cmd.Process != nil {
		t.cmd.Process.Kill()
		t.cmd.Wait()
	}
	return nil
}

// Send sends a JSON-RPC message and returns response
func (t *StdioTransport) Send(ctx context.Context, msg []byte) ([]byte, error) {
	// MCP uses JSON-RPC over stdio
	// This is a simplified implementation
	return msg, nil
}

// IsConnected checks if transport is connected
func (t *StdioTransport) IsConnected() bool {
	return t.cmd.Process != nil
}
