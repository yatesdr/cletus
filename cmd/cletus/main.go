package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cletus/internal/agent"
	"cletus/internal/pipeline"
	"cletus/internal/api"
	"cletus/internal/config"
	"cletus/internal/prompt"
	"cletus/internal/session"
	"cletus/internal/tui"
	"cletus/internal/tools"
	"cletus/internal/tools/sleep"
	"cletus/internal/tools/todowrite"
	"cletus/internal/tools/webfetch"
	"cletus/internal/tools/configtool"
	"cletus/internal/tools/brief"
	"cletus/internal/tools/planmode"
	"cletus/internal/tools/worktree"
	"cletus/internal/tools/lsp"
	"cletus/internal/tools/mcp"
	"cletus/internal/tools/notebook"
	"cletus/internal/tools/repl"
	"cletus/internal/tools/remote"
	"cletus/internal/tools/cron"
	"cletus/internal/tools/sendmessage"
	"cletus/internal/tools/skill"
	"cletus/internal/tools/synthetic"
	"cletus/internal/usage"
	"cletus/internal/cost"
	"cletus/internal/tools/team"
	"cletus/internal/tools/powershell"
	"cletus/internal/tools/taskoutput"
	"cletus/internal/tools/taskstop"
	"cletus/internal/tools/websearch"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

type flags struct {
	ConfigPath   string
	ConfigMDPath string
	Model        string
	Headless     bool
	Prompt       string
	Continue     bool
	SessionID    string
	ShowThinking bool
	APIKey       string
	BaseURL      string
	APIType      string
	Version      bool
}

var cliFlags flags

func parseFlags() *flags {
	flag.StringVar(&cliFlags.ConfigPath, "config", "", "Path to config.json file")
	flag.StringVar(&cliFlags.ConfigMDPath, "config-md", "", "Path to config.md file")
	flag.StringVar(&cliFlags.Model, "model", "", "Model to use")
	flag.BoolVar(&cliFlags.Headless, "headless", false, "Run in headless mode (no TUI)")
	flag.StringVar(&cliFlags.Prompt, "prompt", "", "Prompt to send (headless mode)")
	flag.BoolVar(&cliFlags.Continue, "continue", false, "Continue last session")
	flag.StringVar(&cliFlags.SessionID, "session", "", "Session ID to resume")
	flag.StringVar(&cliFlags.APIKey, "api-key", "", "API key")
	flag.StringVar(&cliFlags.BaseURL, "base-url", "", "API base URL")
	flag.StringVar(&cliFlags.APIType, "api-type", "", "API type (openai or anthropic)")
	flag.BoolVar(&cliFlags.Version, "version", false, "Show version")
	flag.BoolVar(&cliFlags.ShowThinking, "show-thinking", false, "Show thinking blocks in headless mode")
	flag.Parse()

	if cliFlags.Version {
		fmt.Printf("Cletus %s (%s)\n", version, commit)
		os.Exit(0)
	}

	return &cliFlags
}

func main() {
	cfg := parseFlags()

	configMDPath, err := ensureConfigMD(cfg.ConfigMDPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to ensure config.md: %v\n", err)
	}
	cfg.ConfigMDPath = configMDPath

	if _, err := ensureDefaultsMD(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to ensure defaults.md: %v\n", err)
	}

	loadedCfg, err := config.Load(cfg.ConfigPath, cfg.ConfigMDPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load config: %v\n", err)
	} else {
		if cfg.Model != "" {
			loadedCfg.Models.Large = cfg.Model
		}
		if cfg.APIKey != "" {
			loadedCfg.API.APIKey = cfg.APIKey
		}
		if cfg.BaseURL != "" {
			loadedCfg.API.BaseURL = cfg.BaseURL
		}
		if cfg.APIType != "" {
			loadedCfg.API.APIType = cfg.APIType
		}
	}

	apiClient := api.NewLLMClientFromConfig(loadedCfg, "large")
	displayModel := loadedCfg.ResolveModel("large")

	tracker := usage.NewTracker(displayModel)
	sessionStore := session.NewStore("")

	smallModel := loadedCfg.ResolveModel("small")
	compactor := agent.NewCompactor(apiClient, 0.8, 200000, smallModel)

	registry := tools.NewRegistry()
	taskStore := tools.NewTaskStore()
	
	registerTools(registry, taskStore, apiClient, loadedCfg)

	systemPrompt := prompt.BuildFromConfig(loadedCfg)
	fmt.Fprintf(os.Stderr, "System prompt: %d chars\n", len(systemPrompt))
	
	var currentSession *session.Session
	if cliFlags.Continue || cliFlags.SessionID != "" {
		var loadErr error
		if cliFlags.SessionID != "" {
			currentSession, loadErr = sessionStore.Load(cliFlags.SessionID)
		} else {
			currentSession, loadErr = sessionStore.GetLatest()
		}
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load session: %v\n", loadErr)
		} else if currentSession != nil {
			fmt.Fprintf(os.Stderr, "Resuming session: %s\n", currentSession.ID)
		}
	}
	
	if cliFlags.Headless {
		runHeadless(apiClient, registry, loadedCfg, taskStore, tracker, compactor, currentSession, sessionStore, displayModel)
		return
	}

	if !isInteractive() {
		fmt.Fprintf(os.Stderr, "Warning: Not a terminal. Running in headless mode.\n")
		runHeadless(apiClient, registry, loadedCfg, taskStore, tracker, compactor, currentSession, sessionStore, displayModel)
		return
	}

	pl := pipeline.NewPipeline(loadedCfg, apiClient)
	loop := agent.NewLoop(apiClient, registry, loadedCfg, pl)
	loop.SetCompactor(compactor)
	
	if currentSession != nil && len(currentSession.Messages) > 0 {
		loop.SetMessages(currentSession.Messages)
		fmt.Fprintf(os.Stderr, "Restored %d messages from session\n", len(currentSession.Messages))
	}

	workingDir, _ := os.Getwd()

	var app *tui.App
	app = tui.NewApp(tui.Config{
		Model:      displayModel,
		WorkingDir: workingDir,
		Mode:       loadedCfg.Permissions.Mode,
		OnSubmit: func(text string) {
			handleInput(app, loop, text, sessionStore, currentSession, loadedCfg)
		},
		OnQuit: func() {
			if currentSession != nil {
				currentSession.Messages = loop.GetMessages()
				sessionStore.Save(currentSession)
				fmt.Fprintf(os.Stderr, "Session saved: %s\n", currentSession.ID)
			} else if len(loop.GetMessages()) > 0 {
				newSession, _ := sessionStore.Create(displayModel)
				newSession.Messages = loop.GetMessages()
				sessionStore.Save(newSession)
				fmt.Fprintf(os.Stderr, "Session saved: %s\n", newSession.ID)
			}
			app.Stop()
		},
	})

	app.SetSlashCompletions([]string{
		"/help", "/h",
		"/clear", "/c",
		"/model", "/models",
		"/cost", "/cst",
		"/compact",
		"/sessions",
		"/resume",
		"/save",
		"/quit", "/q",
	})

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
	}
}

func runHeadless(client api.LLMClient, registry *tools.Registry, cfg *config.Config, taskStore *tools.TaskStore, tracker *usage.Tracker, compactor *agent.Compactor, currentSession *session.Session, sessionStore *session.Store, displayModel string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	pl := pipeline.NewPipeline(cfg, client)
	loop := agent.NewLoop(client, registry, cfg, pl)
	loop.SetCompactor(compactor)

	if currentSession != nil && len(currentSession.Messages) > 0 {
		loop.SetMessages(currentSession.Messages)
	}

	tracker = usage.NewTracker(displayModel)

	err := loop.RunWithTools(ctx, cliFlags.Prompt, func(event agent.Event) {
		switch event.Type {
		case "message_start":
			fmt.Fprintf(os.Stderr, "[Starting response]\n")
		case "content_block_delta":
			fmt.Print(event.Content)
		case "message_delta":
			tracker.RecordUsage(event.Usage)
		case "message_stop":
			fmt.Fprintf(os.Stderr, "\n[Response complete]\n")
			inputTok, outputTok := loop.GetUsageStats()
			totalTok := inputTok + outputTok
			costVal := cost.CalculateCost(inputTok, outputTok, 0, 0, displayModel)
			fmt.Fprintf(os.Stderr, "Usage: Input=%d, Output=%d, Total=%d tokens\n", inputTok, outputTok, totalTok)
			fmt.Fprintf(os.Stderr, "Estimated cost: %s\n", cost.FormatCost(costVal))
		case "tool_use":
			fmt.Fprintf(os.Stderr, "\n[Tool: %s]\n", event.ToolUse.Name)
		case "tool_result":
			resultPreview := event.ToolResult.Content
			if len(resultPreview) > 200 {
				resultPreview = resultPreview[:200] + "..."
			}
			fmt.Fprintf(os.Stderr, "[Tool result: %s]\n", resultPreview)
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func isInteractive() bool {
	stat, _ := os.Stdout.Stat()
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func handleInput(app *tui.App, loop *agent.Loop, text string, sessionStore *session.Store, currentSession *session.Session, cfg *config.Config) {
	if strings.HasPrefix(text, "/") {
		handleCommand(app, text, loop, sessionStore, currentSession, cfg, cfg.ResolveModel("large"))
		return
	}

	if strings.HasPrefix(text, "!") {
		cmd := strings.TrimPrefix(text, "!")
		app.Chat().AddMessage("user", text)
		go func() {
			out, err := runShellCommand(cmd)
			app.QueueUpdateDraw(func() {
				if err != nil {
					app.Chat().AddMessage("assistant", fmt.Sprintf("Error: %v\n%s", err, out))
				} else {
					app.Chat().AddMessage("assistant", out)
				}
			})
		}()
		return
	}

	// We're on the tview main goroutine here, so update directly (no QueueUpdateDraw)
	app.Chat().AddMessage("user", text)

	// Track current block type to suppress non-text content from the chat view
	thinking := false
	inToolBlock := false

	// Rotating activity phrases shown while Cletus is working
	workPhrases := []string{
		"Chewin' on it...",
		"Cletusing...",
		"Chawin'...",
		"Fixin'...",
		"Scratchin' my head...",
		"Mullin' it over...",
	}
	workPhraseIdx := 0
	nextWorkPhrase := func() string {
		p := workPhrases[workPhraseIdx%len(workPhrases)]
		workPhraseIdx++
		return p
	}

	// Cancellable context for this turn — Escape will call cancel()
	ctx, cancel := context.WithCancel(context.Background())
	app.SetCancelFn(cancel)

	// Run agent loop in background goroutine — all UI updates from here
	// MUST use QueueUpdateDraw since we're off the main goroutine
	go func() {
		defer func() {
			cancel()
			app.SetCancelFn(nil)
		}()
		err := loop.RunWithTools(ctx, text, func(event agent.Event) {
			switch event.Type {
			case "message_start":
				app.QueueUpdateDraw(func() {
					app.Chat().StartStream("assistant")
					app.Status().SetActivity(nextWorkPhrase())
				})

			case "content_block_start":
				if event.ContentBlock != nil {
					switch event.ContentBlock.Type {
					case "thinking":
						thinking = true
						inToolBlock = false
						app.QueueUpdateDraw(func() { app.Status().SetActivity("Scratchin' my head...") })
					case "tool_use":
						inToolBlock = true
					case "text":
						inToolBlock = false
						if thinking {
							thinking = false
							// Start a fresh stream slot for the real content
							app.QueueUpdateDraw(func() {
								app.Chat().StartStream("assistant")
								app.Status().SetActivity("Fixin' to talk...")
							})
						}
					}
				}

			case "content_block_stop":
				thinking = false
				inToolBlock = false

			case "content_block_delta":
				if event.Content != "" && !thinking && !inToolBlock {
					chunk := event.Content
					app.QueueUpdateDraw(func() {
						app.Chat().AppendContent(chunk)
					})
				}

			case "message_stop":
				inputTok, outputTok := loop.GetUsageStats()
				app.QueueUpdateDraw(func() {
					app.Chat().FinishStream()
					app.Status().UpdateTokens(inputTok, outputTok)
					app.Status().ClearActivity()
				})


			case "tool_use":
				if event.ToolUse != nil {
					name := event.ToolUse.Name
					inputStr := string(event.ToolUse.Input)
					app.QueueUpdateDraw(func() {
						app.Chat().AddToolUse(name, inputStr)
						app.Status().SetActivity("Reckon I'll use " + name + "...")
					})
				}

			case "tool_result":
				if event.ToolResult != nil {
					result := event.ToolResult.Content
					isError := event.ToolResult.IsError
					app.QueueUpdateDraw(func() {
						app.Chat().AddToolResult(result, isError)
						app.Status().SetActivity(nextWorkPhrase())
					})
				}

			case "compact_triggered":
				msg := event.Content
				app.QueueUpdateDraw(func() {
					app.Chat().AddMessage("system", msg)
				})

			case "compact_completed":
				msg := event.Content
				app.QueueUpdateDraw(func() {
					app.Chat().AddMessage("system", msg)
				})
			}
		})

		if err != nil {
			if ctx.Err() != nil {
				// Cancelled by the user — finish any open stream and show a notice
				app.QueueUpdateDraw(func() {
					app.Chat().FinishStream()
					app.Chat().AddMessage("system", "cancelled")
				})
			} else {
				errMsg := fmt.Sprintf("Error: %v", err)
				app.QueueUpdateDraw(func() {
					app.Chat().FinishStream()
					app.Chat().AddMessage("assistant", errMsg)
				})
			}
		}
	}()
}

func handleCommand(app *tui.App, text string, loop *agent.Loop, sessionStore *session.Store, currentSession *session.Session, cfg *config.Config, displayModel string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/help", "/h":
		app.Chat().AddMessage("assistant", `Available commands:
/help, /h - Show this help
/clear, /c - Clear chat
/model - Show/set model
/cost, /cst - Show session cost
/compact - Trigger context compaction
/sessions - List saved sessions
/resume [id] - Resume a session
/save - Save current session
/quit, /q - Exit`)
	case "/clear", "/c":
		loop.Reset()
		app.Chat().Clear()
	case "/model", "/models":
		if len(args) > 0 {
			newModel := strings.Join(args, " ")
			cfg.Models.Large = newModel
			app.Status().UpdateModel(newModel)
			app.Chat().AddMessage("system", fmt.Sprintf("Model switched to: %s", newModel))
		} else {
			// Show current model roles and try to list available models from API
			var info strings.Builder
			info.WriteString(fmt.Sprintf("Current model: %s\n\n", displayModel))
			info.WriteString("Configured roles:\n")
			info.WriteString(fmt.Sprintf("  large:        %s\n", cfg.ResolveModel("large")))
			info.WriteString(fmt.Sprintf("  medium:       %s\n", cfg.ResolveModel("medium")))
			info.WriteString(fmt.Sprintf("  small:        %s\n", cfg.ResolveModel("small")))
			info.WriteString(fmt.Sprintf("  vision:       %s\n", cfg.ResolveModel("vision")))
			info.WriteString(fmt.Sprintf("  ocr:          %s\n", cfg.ResolveModel("ocr")))

			// Try to fetch available models from the API
			go func() {
				models, err := fetchAvailableModels(cfg)
				if err != nil {
					app.QueueUpdateDraw(func() {
						app.Chat().AddMessage("assistant", info.String()+"\n(Could not fetch model list: "+err.Error()+")")
					})
					return
				}
				info.WriteString("\nAvailable models on server:\n")
				for _, m := range models {
					info.WriteString(fmt.Sprintf("  - %s\n", m))
				}
				app.QueueUpdateDraw(func() {
					app.Chat().AddMessage("assistant", info.String())
				})
			}()
			return // async — don't fall through
		}
	case "/cost", "/cst":
		inputTok, outputTok := loop.GetUsageStats()
		totalTok := inputTok + outputTok
		costVal := cost.CalculateCost(inputTok, outputTok, 0, 0, displayModel)
		app.Chat().AddMessage("assistant", fmt.Sprintf("Session usage: Input=%d, Output=%d, Total=%d tokens\nEstimated cost: %s", inputTok, outputTok, totalTok, cost.FormatCost(costVal)))
	case "/compact":
		err := loop.Compact()
		if err != nil {
			app.Chat().AddMessage("assistant", fmt.Sprintf("Compaction error: %v", err))
		} else {
			app.Chat().AddMessage("assistant", "Context compacted successfully. Old messages have been summarized.")
		}
	case "/sessions":
		sessions, err := sessionStore.List()
		if err != nil {
			app.Chat().AddMessage("assistant", fmt.Sprintf("Error loading sessions: %v", err))
		} else if len(sessions) == 0 {
			app.Chat().AddMessage("assistant", "No saved sessions found.")
		} else {
			var list []string
			for _, s := range sessions {
				list = append(list, fmt.Sprintf("- %s (%s, %d messages)", s.ID, s.UpdatedAt.Format("2006-01-02 15:04"), len(s.Messages)))
			}
			app.Chat().AddMessage("assistant", "Saved sessions:\n"+strings.Join(list, "\n"))
		}
	case "/resume":
		var sess *session.Session
		var err error
		if len(args) > 0 {
			sess, err = sessionStore.Load(args[0])
		} else {
			sess, err = sessionStore.GetLatest()
		}
		if err != nil {
			app.Chat().AddMessage("assistant", fmt.Sprintf("Error loading session: %v", err))
		} else if sess == nil {
			app.Chat().AddMessage("assistant", "No session found to resume.")
		} else {
			loop.SetMessages(sess.Messages)
			currentSession = sess
			app.Chat().AddMessage("assistant", fmt.Sprintf("Resumed session %s with %d messages", sess.ID, len(sess.Messages)))
		}
	case "/save":
		messages := loop.GetMessages()
		if len(messages) == 0 {
			app.Chat().AddMessage("assistant", "No messages to save.")
		} else {
			if currentSession == nil {
				currentSession, _ = sessionStore.Create(displayModel)
			}
			currentSession.Messages = messages
			sessionStore.Save(currentSession)
			app.Chat().AddMessage("assistant", fmt.Sprintf("Session saved: %s", currentSession.ID))
		}
	case "/quit", "/q":
		app.Stop()
	default:
		app.Chat().AddMessage("assistant", "Unknown command: "+cmd)
	}
}

func registerTools(registry *tools.Registry, taskStore *tools.TaskStore, client api.LLMClient, cfg *config.Config) {
	registry.Register(tools.NewBashTool())
	registry.Register(tools.NewFileReadTool())
	registry.Register(tools.NewFileWriteTool())
	registry.Register(tools.NewFileEditTool())
	registry.Register(tools.NewGlobTool())
	registry.Register(tools.NewGrepTool())
	
	agentTool := tools.NewAgentTool(registry)
	agentTool.SetClient("default", client)
	registry.Register(agentTool)
	registry.Register(tools.NewToolSearchTool(registry))
	registry.Register(tools.NewAskUserQuestionTool(func(q string) string {
		return "User answered: [answer not captured in TUI mode]"
	}))
	registry.Register(tools.NewCreateTaskTool(taskStore))
	registry.Register(tools.NewUpdateTaskTool(taskStore))
	registry.Register(tools.NewListTaskTool(taskStore))
	registry.Register(tools.NewGetTaskTool(taskStore))

	registry.Register(sleep.NewSleepTool())
	registry.Register(todowrite.NewTodoWriteTool())
	registry.Register(webfetch.NewWebFetchTool())
	registry.Register(websearch.NewWebSearchTool(cfg.WebSearchKey))

	registry.Register(taskstop.NewTaskStopTool())
	registry.Register(taskoutput.NewTaskOutputTool())

	registry.Register(configtool.NewConfigTool())
	registry.Register(powershell.NewPowerShellTool())

	registry.Register(brief.NewBriefTool())
	registry.Register(planmode.NewEnterPlanModeTool())
	registry.Register(planmode.NewExitPlanModeTool())
	registry.Register(worktree.NewEnterWorktreeTool())
	registry.Register(worktree.NewExitWorktreeTool())
	registry.Register(lsp.NewLSPTool())
	registry.Register(mcp.NewListMcpResourcesTool())
	registry.Register(mcp.NewReadMcpResourceTool())
	registry.Register(mcp.NewMcpAuthTool())
	registry.Register(notebook.NewNotebookEditTool())
	registry.Register(repl.NewREPLTool())
	registry.Register(remote.NewRemoteTriggerTool())
	registry.Register(cron.NewScheduleCronTool())
	registry.Register(sendmessage.NewSendMessageTool())
	registry.Register(skill.NewSkillTool())
	registry.Register(synthetic.NewSyntheticOutputTool())
	registry.Register(team.NewTeamCreateTool())
	registry.Register(team.NewTeamDeleteTool())
}

// fetchAvailableModels queries the API for available models
func fetchAvailableModels(cfg *config.Config) ([]string, error) {
	url := cfg.API.BaseURL + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if cfg.API.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.API.APIKey)
		req.Header.Set("x-api-key", cfg.API.APIKey)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range result.Data {
		models = append(models, m.ID)
	}
	return models, nil
}

// runShellCommand executes a shell command and returns combined stdout+stderr output
func runShellCommand(cmd string) (string, error) {
	c := exec.Command("sh", "-c", cmd)
	out, err := c.CombinedOutput()
	return string(out), err
}
