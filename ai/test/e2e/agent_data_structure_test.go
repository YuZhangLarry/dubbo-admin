/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package e2e

import (
	"encoding/json"
	"strings"
	"testing"

	"dubbo-admin-ai/component/tools/engine"
	"dubbo-admin-ai/schema"
)

// TestAgentStateStructure tests AgentState structure and methods
func TestAgentStateStructure(t *testing.T) {
	t.Run("Basic State Creation", func(t *testing.T) {
		state := &schema.AgentState{
			UserQuery: "测试查询",
			SessionID: "test_session",
			Iteration: 0,
		}

		if state.UserQuery != "测试查询" {
			t.Errorf("UserQuery mismatch: got %s, want %s", state.UserQuery, "测试查询")
		}
		if state.SessionID != "test_session" {
			t.Errorf("SessionID mismatch: got %s, want %s", state.SessionID, "test_session")
		}
		if !state.IsFirstIteration() {
			t.Error("Should be first iteration")
		}
	})

	t.Run("Iteration Tracking", func(t *testing.T) {
		state := &schema.AgentState{
			Iteration: 0,
		}

		if !state.IsFirstIteration() {
			t.Error("Iteration 0 should be first")
		}

		state.Iteration = 1
		if state.IsFirstIteration() {
			t.Error("Iteration 1 should not be first")
		}

		state.Iteration = 5
		if state.IsFirstIteration() {
			t.Error("Iteration 5 should not be first")
		}
	})

	t.Run("Tool Results Management", func(t *testing.T) {
		state := &schema.AgentState{}

		// Initially no tool results
		if state.HasToolResult() {
			t.Error("Should not have tool result initially")
		}

		outputs := state.GetToolOutputs()
		if len(outputs) != 0 {
			t.Error("Initial tool outputs should be empty")
		}

		// Add tool results
		toolOutputs := []engine.ToolOutput{
			{ToolName: "test_tool", Result: map[string]any{"result": "success"}},
		}
		state.LastToolResult = &schema.ToolOutputs{
			Outputs: toolOutputs,
		}

		if !state.HasToolResult() {
			t.Error("Should have tool result after setting")
		}

		retrieved := state.GetToolOutputs()
		if len(retrieved) != len(toolOutputs) {
			t.Errorf("Tool outputs count mismatch: got %d, want %d", len(retrieved), len(toolOutputs))
		}
	})
}

// TestThinkStageInputSerialization tests Think stage input JSON serialization
func TestThinkStageInputSerialization(t *testing.T) {
	input := schema.ThinkStageInput{
		UserQuery: "测试查询",
		SessionID: "test_session",
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify JSON is valid
	var decoded schema.ThinkStageInput
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.UserQuery != input.UserQuery {
		t.Error("UserQuery mismatch after round-trip")
	}
	if decoded.SessionID != input.SessionID {
		t.Error("SessionID mismatch after round-trip")
	}
}

// TestActStageInputSerialization tests Act stage input JSON serialization
func TestActStageInputSerialization(t *testing.T) {
	input := schema.ActStageInput{
		UserQuery:      "查询应用",
		SuggestedTools: []string{"application_list", "search_service"},
		SessionID:      "test_session",
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded schema.ActStageInput
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(decoded.SuggestedTools) != len(input.SuggestedTools) {
		t.Errorf("SuggestedTools count mismatch: got %d, want %d", len(decoded.SuggestedTools), len(input.SuggestedTools))
	}

	// Verify JSON string representation
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, "application_list") {
		t.Error("JSON should contain application_list")
	}
}

// TestObserveStageInputSerialization tests Observe stage input JSON serialization
func TestObserveStageInputSerialization(t *testing.T) {
	input := schema.ObserveStageInput{
		UserQuery:    "查询应用",
		Intent:       schema.GeneralInquiry,
		ThinkThought: "需要查询应用列表",
	}

	jsonBytes, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded schema.ObserveStageInput
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Intent != input.Intent {
		t.Errorf("Intent mismatch: got %s, want %s", decoded.Intent, input.Intent)
	}

	// Verify JSON string
	jsonStr := string(jsonBytes)
	if !strings.Contains(jsonStr, "需要查询应用列表") {
		t.Error("JSON should contain ThinkThought")
	}
}

// TestThinkOutputStructure tests Think output structure
func TestThinkOutputStructure(t *testing.T) {
	output := &schema.ThinkOutput{
		Intent: schema.GeneralInquiry,
		Thought: "用户想查询信息",
		SuggestedTools: []string{"search"},
	}

	jsonBytes, err := json.Marshal(output)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded schema.ThinkOutput
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Intent != schema.GeneralInquiry {
		t.Error("Intent mismatch")
	}
	if len(decoded.SuggestedTools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(decoded.SuggestedTools))
	}
}

// TestObservationStructure tests Observation structure
func TestObservationStructure(t *testing.T) {
	observation := &schema.Observation{
		Summary:     "查询完成",
		FinalAnswer: "找到了3个应用",
		Heartbeat:   false,
	}

	jsonBytes, err := json.Marshal(observation)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded schema.Observation
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.FinalAnswer != observation.FinalAnswer {
		t.Error("FinalAnswer mismatch")
	}
	if decoded.Heartbeat {
		t.Error("Heartbeat should be false")
	}
}

// TestPrimaryIntentValues tests primary intent enum values
func TestPrimaryIntentValues(t *testing.T) {
	intents := []schema.PrimaryIntent{
		schema.PerformanceInvestigation,
		schema.ErrorDiagnosis,
		schema.HealthCheck,
		schema.ResourceMonitoring,
		schema.TrafficAnalysis,
		schema.ServiceDependency,
		schema.AlertingInvestigation,
		schema.MemorySearch,
		schema.GeneralInquiry,
	}

	for _, intent := range intents {
		if intent == "" {
			t.Error("Intent should not be empty")
		}
	}
}

// TestDataFlowIntegration tests end-to-end data flow simulation
func TestDataFlowIntegration(t *testing.T) {
	t.Run("Full Flow Simulation", func(t *testing.T) {
		// Simulate user query
		userQuery := "查询所有应用列表"

		// Stage 1: Think
		thinkInput := schema.ThinkStageInput{
			UserQuery: userQuery,
			SessionID: "test_123",
		}
		thinkJSON, _ := json.Marshal(thinkInput)
		t.Logf("Think Input JSON: %s", string(thinkJSON))

		// Simulate Think output
		thinkOutput := &schema.ThinkOutput{
			Intent: schema.GeneralInquiry,
			Thought: "用户想要查询应用列表，需要调用工具",
			SuggestedTools: []string{"application_list"},
		}

		// Stage 2: Act
		actInput := schema.ActStageInput{
			UserQuery:      userQuery,
			SuggestedTools: thinkOutput.SuggestedTools,
			SessionID:      "test_123",
		}
		actJSON, _ := json.Marshal(actInput)
		t.Logf("Act Input JSON: %s", string(actJSON))

		// Simulate Act output
		actOutput := schema.ToolOutputs{
			Outputs: []engine.ToolOutput{
				{ToolName: "application_list", Result: map[string]any{"applications": []string{"app1", "app2"}}},
			},
		}

		// Stage 3: Observe
		observeInput := schema.ObserveStageInput{
			UserQuery:    userQuery,
			Intent:       thinkOutput.Intent,
			ThinkThought: thinkOutput.Thought,
		}
		if len(actOutput.Outputs) > 0 {
			observeInput.ToolResponse = actOutput.Outputs
		}
		observeJSON, _ := json.Marshal(observeInput)
		t.Logf("Observe Input JSON: %s", string(observeJSON))

		// Simulate Observation
		observation := &schema.Observation{
			Summary:     "查询到2个应用",
			FinalAnswer: "找到了2个应用：app1, app2",
			Heartbeat:   false,
		}
		observeJSON, _ = json.Marshal(observation)
		t.Logf("Final Observation: %s", string(observeJSON))

		// Verify data flow
		if observation.FinalAnswer == "" {
			t.Error("Final answer should not be empty")
		}
		if observation.Heartbeat {
			t.Error("Should not be heartbeat for final answer")
		}
	})
}
