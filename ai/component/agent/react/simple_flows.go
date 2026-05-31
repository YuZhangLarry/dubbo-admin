package react

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"dubbo-admin-ai/component/agent/fallback"
	"dubbo-admin-ai/component/tools/engine"
	"dubbo-admin-ai/runtime"
	"dubbo-admin-ai/schema"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// Global fallback handler (can be configured)
var globalFallback *fallback.Handler

func init() {
	globalFallback = fallback.NewHandler(fallback.DefaultFallbackConfig())
}

// SetFallbackConfig sets the global fallback configuration
func SetFallbackConfig(config *fallback.FallbackConfig) {
	globalFallback = fallback.NewHandler(config)
}

// SimpleThinkFunc creates a Think function that uses AgentState.
// Uses structured input instead of conversation history.
func SimpleThinkFunc(
	g *genkit.Genkit,
	thinkPrompt ai.Prompt,
) func(ctx context.Context, state *schema.AgentState) (*schema.ThinkOutput, error) {
	return func(ctx context.Context, state *schema.AgentState) (*schema.ThinkOutput, error) {
		runtime.GetLogger().Info("SimpleThink: starting", "iteration", state.Iteration)
		defer func() {
			runtime.GetLogger().Info("SimpleThink: done", "iteration", state.Iteration)
		}()

		// Build structured input for Think
		thinkIn := schema.ThinkStageInput{
			UserQuery:     state.UserQuery,
			ToolResponses: state.GetToolOutputs(),
			SessionID:     state.SessionID,
		}

		// Marshal input to JSON
		inputJson, err := json.Marshal(thinkIn)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal think input: %w", err)
		}

		// Add timeout protection
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		// Create user message with JSON input
		userMsg := ai.NewUserMessage(ai.NewJSONPart(string(inputJson)))

		resp, err := thinkPrompt.Execute(ctx, ai.WithMessages(userMsg))
		if err != nil {
			runtime.GetLogger().Error("Think prompt execution failed", "error", err)
			return globalFallback.DefaultThinkOutput(fmt.Sprintf("execute error: %v", err)), nil
		}
		if resp == nil {
			runtime.GetLogger().Error("Think prompt returned nil response")
			return globalFallback.DefaultThinkOutput("nil response"), nil
		}

		// Parse output with fallback
		thinkOut, err := globalFallback.ParseThinkOutput(resp)
		if err != nil {
			return globalFallback.DefaultThinkOutput(fmt.Sprintf("parse error: %v", err)), nil
		}

		runtime.GetLogger().Info("SimpleThink: result", "thought", thinkOut.Thought, "tools", thinkOut.SuggestedTools)

		return thinkOut, nil
	}
}

// SimpleActFunc creates an Act function that uses LLM to generate tool calls.
// Uses structured input instead of conversation history.
func SimpleActFunc(
	g *genkit.Genkit,
	actPrompt ai.Prompt,
) func(ctx context.Context, state *schema.AgentState) (schema.ToolOutputs, error) {
	return func(ctx context.Context, state *schema.AgentState) (schema.ToolOutputs, error) {
		runtime.GetLogger().Info("SimpleAct: starting")
		defer func() {
			runtime.GetLogger().Info("SimpleAct: done")
		}()

		actOut := schema.ToolOutputs{
			Outputs:   []engine.ToolOutput{},
			UsageInfo: &ai.GenerationUsage{},
		}

		thinkOut := state.ThinkResult
		if thinkOut == nil {
			return actOut, fmt.Errorf("think result is nil")
		}

		runtime.GetLogger().Info("SimpleAct: executing tools", "suggested_tools", thinkOut.SuggestedTools)

		// Build structured input for Act
		actIn := schema.ActStageInput{
			UserQuery:      state.UserQuery,
			SuggestedTools: thinkOut.SuggestedTools,
			SessionID:      state.SessionID,
		}

		// Marshal input to JSON
		inputJson, err := json.Marshal(actIn)
		if err != nil {
			return actOut, fmt.Errorf("failed to marshal act input: %w", err)
		}

		// Add timeout protection
		ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		// Create user message with JSON input
		userMsg := ai.NewUserMessage(ai.NewJSONPart(string(inputJson)))

		toolReqs, err := actPrompt.Execute(ctx, ai.WithMessages(userMsg))
		if err != nil {
			runtime.GetLogger().Error("Act prompt execution failed", "error", err)
			return actOut, fmt.Errorf("act execution failed: %w", err)
		}

		toolReqList := toolReqs.ToolRequests()
		runtime.GetLogger().Info("SimpleAct: tool requests from LLM", "count", len(toolReqList))

		if len(toolReqList) == 0 {
			// Model decided not to use suggested tools - this is normal behavior
			runtime.GetLogger().Info("SimpleAct: LLM chose not to call tools", "suggested_tools", thinkOut.SuggestedTools)
			actOut.Thought = "Model determined no tools were needed"
			return actOut, nil
		}

		// Execute each tool request
		for _, req := range toolReqList {
			runtime.GetLogger().Info("SimpleAct: calling tool", "tool", req.Name, "input", req.Input)

			output, err := engine.Call(g, req.Name, req.Input)
			if err != nil {
				return actOut, fmt.Errorf("failed to call tool %s: %w", req.Name, err)
			}

			actOut.Outputs = append(actOut.Outputs, output)
		}

		return actOut, nil
	}
}

// SimpleObserveFunc creates an Observe function that uses AgentState.
// Uses structured input instead of conversation history.
func SimpleObserveFunc(
	g *genkit.Genkit,
	observePrompt ai.Prompt,
) func(ctx context.Context, state *schema.AgentState) (*schema.Observation, error) {
	return func(ctx context.Context, state *schema.AgentState) (*schema.Observation, error) {
		runtime.GetLogger().Info("SimpleObserve: starting", "iteration", state.Iteration)
		defer func() {
			runtime.GetLogger().Info("SimpleObserve: done")
		}()

		// Build structured input for Observe
		observeIn := schema.ObserveStageInput{
			UserQuery: state.UserQuery,
		}

		// Add tool results if available
		if state.LastToolResult != nil && len(state.LastToolResult.Outputs) > 0 {
			observeIn.ToolResponse = state.LastToolResult.Outputs
		}

		// Add intent from Think stage
		if state.ThinkResult != nil {
			observeIn.Intent = state.ThinkResult.Intent
			observeIn.ThinkThought = state.ThinkResult.Thought
		}

		// Marshal input to JSON
		inputJson, err := json.Marshal(observeIn)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal observe input: %w", err)
		}
		runtime.GetLogger().Info("SimpleObserve: input prepared", "json_len", len(inputJson))

		// Add timeout protection
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		// Create user message with JSON input
		userMsg := ai.NewUserMessage(ai.NewJSONPart(string(inputJson)))

		runtime.GetLogger().Info("SimpleObserve: calling LLM")
		resp, err := observePrompt.Execute(ctx, ai.WithMessages(userMsg))
		runtime.GetLogger().Info("SimpleObserve: LLM returned", "error", err)
		if err != nil {
			runtime.GetLogger().Error("Observe prompt execution failed", "error", err)
			return globalFallback.DefaultObservation(fmt.Sprintf("execute error: %v", err), state.UserQuery), nil
		}
		if resp == nil {
			runtime.GetLogger().Error("Observe prompt returned nil response")
			return globalFallback.DefaultObservation("nil response", state.UserQuery), nil
		}

		// Parse output with fallback
		observation, err := globalFallback.ParseObservation(resp)
		if err != nil {
			return globalFallback.DefaultObservation(fmt.Sprintf("parse error: %v", err), state.UserQuery), nil
		}

		runtime.GetLogger().Info("SimpleObserve: result", "summary", observation.Summary, "final_answer", observation.FinalAnswer)

		return observation, nil
	}
}
