package react

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"dubbo-admin-ai/component/agent"
	"dubbo-admin-ai/component/memory"
	"dubbo-admin-ai/runtime"
	"dubbo-admin-ai/schema"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// ReActAgent implements a ReAct loop using AgentState.
type ReActAgent struct {
	registry       *genkit.Genkit
	memoryCtx      context.Context
	channels       *agent.Channels
	orchestrator   *agent.SimpleOrchestrator
	defaultModel   string
	promptBasePath string
	maxIterations  int
}

// NewReActAgent creates a new ReActAgent.
func NewReActAgent(
	g *genkit.Genkit,
	promptBasePath string,
	defaultModel string,
	maxIterations int,
	stagesCfg []StageInfo,
	toolRefs []ai.ToolRef,
) (*ReActAgent, error) {
	memoryCtx := memory.NewMemoryContext(memory.ChatHistoryKey)
	channels := agent.NewChannels(len(stagesCfg))

	// Create SimpleOrchestrator
	orchestrator := agent.NewSimpleOrchestrator(g, maxIterations, memoryCtx)

	// Build stages and configure orchestrator
	for _, stageCfg := range stagesCfg {
		// Read and build prompt
		promptPath := path.Join(promptBasePath, stageCfg.PromptFile)
		systemPrompt, err := os.ReadFile(promptPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read prompt file %s: %w", promptPath, err)
		}

		// Prepare tools
		var tools []ai.ToolRef
		if stageCfg.EnableTools {
			tools = toolRefs
		}

		// Prepare available tools names for think stage
		extraPrompt := stageCfg.ExtraPrompt
		if stageCfg.FlowType == "think" && extraPrompt == "" {
			toolNames := make([]string, 0, len(toolRefs))
			for _, toolRef := range toolRefs {
				toolNames = append(toolNames, toolRef.Name())
			}
			toolsJson, err := json.Marshal(toolNames)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool names: %w", err)
			}
			extraPrompt = fmt.Sprintf("available tools: %s", string(toolsJson))
			runtime.GetLogger().Debug("Tool details", "extraPrompt", extraPrompt)
		}

		// Use default model if not specified
		model := stageCfg.Model
		if model == "" {
			model = defaultModel
		}

		// Build prompt
		prompt, err := buildPromptForStage(stageCfg.FlowType, g, systemPrompt, stageCfg.Temperature, model, extraPrompt, tools...)
		if err != nil {
			return nil, fmt.Errorf("failed to build prompt for stage %s: %w", stageCfg.Name, err)
		}

		// Configure orchestrator with stage functions
		switch stageCfg.FlowType {
		case "think":
			orchestrator.SetThinkFunc(SimpleThinkFunc(g, prompt))
		case "act":
			orchestrator.SetActFunc(SimpleActFunc(g, prompt))
		case "observe":
			orchestrator.SetObserveFunc(SimpleObserveFunc(g, prompt))
		default:
			return nil, fmt.Errorf("unknown flow type: %s", stageCfg.FlowType)
		}
	}

	return &ReActAgent{
		registry:       g,
		orchestrator:   orchestrator,
		memoryCtx:      memoryCtx,
		channels:       channels,
		defaultModel:   defaultModel,
		promptBasePath: promptBasePath,
		maxIterations:  maxIterations,
	}, nil
}

// Interact starts a new interaction with the agent.
func (ra *ReActAgent) Interact(input *schema.UserInput, sessionID string) *agent.Channels {
	ra.channels.Reset()
	go func() {
		_, err := ra.orchestrator.RunSimple(ra.memoryCtx, input.Content, sessionID, ra.channels)
		if err != nil {
			ra.channels.ErrorChan <- err
		}
		ra.channels.Close()
	}()
	return ra.channels
}

// GetMemory returns the history memory.
func (ra *ReActAgent) GetMemory() *memory.HistoryMemory {
	h, _ := memory.GetHistoryMemory(ra.memoryCtx, memory.ChatHistoryKey)
	return h
}

// buildPromptForStage builds a prompt for a specific stage type.
func buildPromptForStage(flowType string, registry *genkit.Genkit, systemPrompt []byte, temp float64, model string, extraPrompt string, tools ...ai.ToolRef) (ai.Prompt, error) {
	opts := []ai.PromptOption{
		ai.WithSystem(string(systemPrompt)),
		ai.WithConfig(nil),
		ai.WithModelName(model),
	}
	if extraPrompt != "" {
		opts = append(opts, ai.WithPrompt(extraPrompt))
	}
	if tools != nil {
		opts = append(opts, ai.WithTools(tools...), ai.WithReturnToolRequests(true))
	}

	// Set input/output types based on flow type
	switch flowType {
	case "think":
		opts = append(opts, ai.WithInputType(schema.ThinkInput{}))
		opts = append(opts, ai.WithOutputType(schema.ThinkOutput{}))
	case "act":
		// Act generates tool calls - no input/output type constraints
		// This allows LLM to freely decide which tools to call
	case "observe":
		opts = append(opts, ai.WithOutputType(schema.Observation{}))
	}

	return genkit.DefinePrompt(registry, flowType, opts...), nil
}
