package agent

import (
	"context"
	"errors"
	"fmt"

	"dubbo-admin-ai/component/agent/fallback"
	"dubbo-admin-ai/component/memory"
	"dubbo-admin-ai/runtime"
	"dubbo-admin-ai/schema"

	"github.com/firebase/genkit/go/genkit"
)

// SimpleOrchestrator implements a simplified ReAct loop using AgentState.
// It uses structured input/output for each stage instead of conversation history.
type SimpleOrchestrator struct {
	g            *genkit.Genkit
	maxIteration int

	// Stage functions
	thinkFunc func(ctx context.Context, state *schema.AgentState) (*schema.ThinkOutput, error)
	actFunc   func(ctx context.Context, state *schema.AgentState) (schema.ToolOutputs, error)
	observeFunc func(ctx context.Context, state *schema.AgentState) (*schema.Observation, error)

	// Memory for persistence (for turn management)
	memoryCtx context.Context

	// Fallback handler for serialization and loop control
	fallback *fallback.Handler
}

// NewSimpleOrchestrator creates a new SimpleOrchestrator.
func NewSimpleOrchestrator(
	g *genkit.Genkit,
	maxIteration int,
	memoryCtx context.Context,
) *SimpleOrchestrator {
	return &SimpleOrchestrator{
		g:            g,
		maxIteration: maxIteration,
		memoryCtx:    memoryCtx,
		fallback:     fallback.NewHandler(fallback.DefaultFallbackConfig()),
	}
}

// SetThinkFunc sets the Think stage function.
func (o *SimpleOrchestrator) SetThinkFunc(fn func(ctx context.Context, state *schema.AgentState) (*schema.ThinkOutput, error)) {
	o.thinkFunc = fn
}

// SetActFunc sets the Act stage function.
func (o *SimpleOrchestrator) SetActFunc(fn func(ctx context.Context, state *schema.AgentState) (schema.ToolOutputs, error)) {
	o.actFunc = fn
}

// SetObserveFunc sets the Observe stage function.
func (o *SimpleOrchestrator) SetObserveFunc(fn func(ctx context.Context, state *schema.AgentState) (*schema.Observation, error)) {
	o.observeFunc = fn
}

// sendResult sends the observation result to the user and ends the turn
func (o *SimpleOrchestrator) sendResult(channels *Channels, observation *schema.Observation) {
	if observation == nil {
		return
	}
	if observation.Summary != "" {
		channels.UserRespChan <- schema.NewStreamFeedback(observation.Summary + "\n")
	}
	if observation.FinalAnswer != "" {
		channels.UserRespChan <- schema.NewStreamFeedback(observation.FinalAnswer + "\n")
	}
	channels.UserRespChan <- schema.StreamEnd()
}

// RunSimple executes the ReAct loop with AgentState using structured input/output.
func (o *SimpleOrchestrator) RunSimple(ctx context.Context, userQuery string, sessionID string, channels *Channels) (*schema.Observation, error) {
	runtime.GetLogger().Info("Agent orchestration started", "sessionID", sessionID)

	if o.thinkFunc == nil {
		return nil, errors.New("think function not set")
	}
	if o.actFunc == nil {
		return nil, errors.New("act function not set")
	}
	if o.observeFunc == nil {
		return nil, errors.New("observe function not set")
	}

	// Get memory for turn management (no longer used for conversation history)
	history, ok := o.memoryCtx.Value(memory.ChatHistoryKey).(*memory.HistoryMemory)
	if !ok {
		return nil, fmt.Errorf("failed to get history from context")
	}

	// Initialize AgentState with structured data
	state := &schema.AgentState{
		UserQuery: userQuery,
		SessionID: sessionID,
		Iteration: 0,
		// ConversationHistory is no longer used, kept for compatibility
		ConversationHistory: nil,
	}

	// Track consecutive iterations without tools - prevent infinite loops
	consecutiveNoTools := 0

	// ReAct loop
	var finalObservation *schema.Observation
	for state.Iteration < o.maxIteration {
		runtime.GetLogger().Info("ReAct iteration started", "iteration", state.Iteration, "sessionID", sessionID)

		// Think stage
		runtime.GetLogger().Info("Think stage started", "iteration", state.Iteration)
		emitStageProgress(channels, ThinkFlowName, true)
		thinkOut, err := o.thinkFunc(ctx, state)
		if err != nil {
			runtime.GetLogger().Error("Think stage failed", "iteration", state.Iteration, "error", err)
			return nil, fmt.Errorf("think stage failed: %w", err)
		}
		runtime.GetLogger().Info("Think stage completed", "iteration", state.Iteration, "intent", thinkOut.Intent, "tools", len(thinkOut.SuggestedTools))

		// Think output is internal decision, don't add to history (LLM doesn't need it)
		state.ThinkResult = thinkOut
		emitStageProgress(channels, ThinkFlowName, false)

		// Check if Think suggests no tools
		// Skip Act only when there are truly no tools to execute
		if len(thinkOut.SuggestedTools) == 0 {
			consecutiveNoTools++
			runtime.GetLogger().Info("No tools suggested, skipping Act stage", "iteration", state.Iteration, "intent", thinkOut.Intent, "consecutive_no_tools", consecutiveNoTools)

			// Check if we should force fallback using centralized logic
			if shouldFallback, reason := o.fallback.ShouldForceFallback(consecutiveNoTools, state.Iteration, o.maxIteration); shouldFallback {
				runtime.GetLogger().Warn("Forcing fallback due to loop conditions", "reason", reason)
				fallbackObs := o.fallback.NoToolsFallback(string(thinkOut.Intent), state.UserQuery)
				o.sendResult(channels, fallbackObs)
				history.NextTurn(sessionID)
				return fallbackObs, nil
			}

			// No tools needed, go straight to Observe
		} else {
			// Reset counter when we have tools
			consecutiveNoTools = 0

			// Act stage
			runtime.GetLogger().Info("Act stage started", "iteration", state.Iteration, "tools", thinkOut.SuggestedTools)
			emitStageProgress(channels, ActFlowName, true)
			toolOut, err := o.actFunc(ctx, state)
			if err != nil {
				runtime.GetLogger().Error("Act stage failed", "iteration", state.Iteration, "error", err)
				// Act failed - generate fallback observation and end
				fallbackObs := o.fallback.DefaultObservation(fmt.Sprintf("Tool execution failed: %v", err), state.UserQuery)
				o.sendResult(channels, fallbackObs)
				history.NextTurn(sessionID)
				return fallbackObs, nil
			}
			runtime.GetLogger().Info("Act stage completed", "iteration", state.Iteration, "tool_outputs", len(toolOut.Outputs))

			// Handle Act results
			if len(toolOut.Outputs) == 0 {
				// LLM chose not to call tools
				runtime.GetLogger().Info("Act returned no tools")
				// Count as consecutive no-tools to trigger fallback
				consecutiveNoTools++
				if shouldFallback, reason := o.fallback.ShouldForceFallback(consecutiveNoTools, state.Iteration, o.maxIteration); shouldFallback {
					runtime.GetLogger().Warn("Forcing fallback after LLM refused tools", "reason", reason)
					fallbackObs := o.fallback.NoToolsFallback(string(thinkOut.Intent), state.UserQuery)
					o.sendResult(channels, fallbackObs)
					history.NextTurn(sessionID)
					return fallbackObs, nil
				}
			} else {
				// Successfully executed tools, reset counter
				consecutiveNoTools = 0
			}

			state.LastToolResult = &toolOut
			emitStageProgress(channels, ActFlowName, false)
		}

		// Observe stage
		runtime.GetLogger().Info("Observe stage started", "iteration", state.Iteration)
		emitStageProgress(channels, ObserveFlowName, true)
		observation, err := o.observeFunc(ctx, state)
		if err != nil {
			runtime.GetLogger().Error("Observe stage failed", "iteration", state.Iteration, "error", err)
			return nil, fmt.Errorf("observe stage failed: %w", err)
		}
		runtime.GetLogger().Info("Observe stage completed", "iteration", state.Iteration, "heartbeat", observation.Heartbeat, "has_answer", observation.FinalAnswer != "")

		// Observation is internal decision, don't add to history (LLM doesn't need it)
		state.ObservationResult = observation
		emitStageProgress(channels, ObserveFlowName, false)

		// Check if we have a final answer
		if !observation.Heartbeat && observation.FinalAnswer != "" {
			runtime.GetLogger().Info("Final answer received, ending loop", "iteration", state.Iteration)
			finalObservation = observation
			break
		}

		state.Iteration++
	}

	// If no final observation was set, use fallback
	if finalObservation == nil {
		if state.ObservationResult != nil && state.ObservationResult.FinalAnswer != "" {
			runtime.GetLogger().Info("Using last observation as final answer")
			finalObservation = state.ObservationResult
		} else {
			// Fallback: reached max iterations without a proper answer
			runtime.GetLogger().Warn("Reached max iterations without final answer, using fallback")
			finalObservation = o.fallback.MaxIterationsFallback(state.Iteration, state.UserQuery)
		}
	}

	// Send final result to user
	o.sendResult(channels, finalObservation)

	// Next turn (for memory management)
	history.NextTurn(sessionID)
	runtime.GetLogger().Info("Agent orchestration completed", "sessionID", sessionID, "total_iterations", state.Iteration)

	return finalObservation, nil
}
