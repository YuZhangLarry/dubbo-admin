package schema

import (
	"dubbo-admin-ai/component/tools/engine"

	"github.com/firebase/genkit/go/ai"
)

// AgentState holds the iteration state for the ReAct loop.
// It is ephemeral (in-memory) and not persisted between requests.
type AgentState struct {
	// UserQuery is the original user input
	UserQuery string

	// SessionID identifies the conversation session
	SessionID string

	// Iteration counts the current loop iteration (0-indexed)
	Iteration int

	// LastToolResult holds the output from the previous Act stage.
	// On the first iteration, this is nil.
	LastToolResult *ToolOutputs

	// ThinkResult holds the output from the previous Think stage.
	ThinkResult *ThinkOutput

	// ObservationResult holds the output from the previous Observe stage.
	ObservationResult *Observation

	// ConversationHistory is the conversation history loaded from Memory.
	// This is used for context when needed.
	ConversationHistory []*ai.Message
}

// IsFirstIteration returns true if this is the first ReAct iteration.
func (s *AgentState) IsFirstIteration() bool {
	return s.Iteration == 0
}

// HasToolResult returns true if there's a tool result from the previous iteration.
func (s *AgentState) HasToolResult() bool {
	return s.LastToolResult != nil
}

// GetToolOutputs converts the last tool result to a slice for compatibility.
func (s *AgentState) GetToolOutputs() []engine.ToolOutput {
	if s.LastToolResult == nil {
		return []engine.ToolOutput{}
	}
	return s.LastToolResult.Outputs
}

// ThinkStageInput is the structured input for the Think stage.
type ThinkStageInput struct {
	UserQuery     string              `json:"user_query"`
	ToolResponses []engine.ToolOutput `json:"tool_responses,omitempty"`
	SessionID     string              `json:"session_id"`
}

// ActStageInput is the structured input for the Act stage.
type ActStageInput struct {
	UserQuery      string   `json:"user_query"`
	SuggestedTools []string `json:"suggested_tools"`
	SessionID      string   `json:"session_id"`
}

// ObserveStageInput is the structured input for the Observe stage.
type ObserveStageInput struct {
	UserQuery    string                `json:"user_query"`
	ToolResponse []engine.ToolOutput   `json:"tool_response,omitempty"`
	Intent       PrimaryIntent         `json:"intent"`
	ThinkThought string                `json:"think_thought"`
}
