# cletus

A Go-based AI coding assistant for the terminal. Single binary, works with any OpenAI-compatible or Anthropic API endpoint. Designed for local models via vLLM, SGLang, Ollama, llama-server, or go-llm-proxy.

## Features

- **Dual-native API support** — speaks OpenAI Chat Completions and Anthropic Messages API natively
- **40+ built-in tools** — file read/write/edit, bash, glob, grep, web search, MCP, and more
- **Agent loop** — streams responses, executes tools, feeds results back automatically
- **Generic model roles** — configure `large`, `medium`, `small`, `vision`, `ocr` models independently
- **Per-model backends** — point different models at different servers
- **Content pipeline** — vision description, PDF text extraction, OCR for text-only models
- **Terminal UI** — tview-based with streaming, markdown rendering, permission dialogs
- **Headless mode** — use in scripts and pipelines with `-headless -prompt "..."`
- **Session persistence** — save/resume conversations
- **Permission system** — configurable rules for tool approval
- **Hook system** — shell commands on tool events
- **Memory system** — persistent context across sessions
- **CLETUS.md support** — project-specific instructions

## Quick Start

```bash
# Build
make build

# Run with a local model
./cletus -base-url http://localhost:8080/v1 -api-key your-key -model your-model

# Run in headless mode
./cletus -headless -prompt "list files in the current directory"

# Use Anthropic API directly
./cletus -base-url https://api.anthropic.com/v1 -api-key sk-ant-xxx -api-type anthropic -model claude-sonnet-4-6
```

## Configuration

Copy `settings.json.example` to `~/.config/cletus/settings.json` and edit:

```json
{
  "api": {
    "base_url": "http://localhost:8080/v1",
    "api_key": "your-key",
    "api_type": "openai"
  },
  "models": {
    "large": "MiniMax-M2.5",
    "small": "glm-5-turbo",
    "vision": "MiniMax-M2.5",
    "ocr": "paddleOCR"
  }
}
```

## Building

```bash
make build        # Build for current platform
make test         # Run tests
make all          # Build for all platforms (linux, macOS, windows)
make clean        # Remove build artifacts
```

## License

MIT
