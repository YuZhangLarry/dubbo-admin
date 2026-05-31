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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// APIIntegrationTest tests the AI service by calling HTTP API directly
// Assumes the AI service is already running on http://127.0.0.1:8880
func APIIntegrationTest(t *testing.T) {
	baseURL := "http://127.0.0.1:8880"
	client := &http.Client{Timeout: 120 * time.Second}

	// Test cases
	testCases := []struct {
		ID       string
		Query    string
		Category string
	}{
		{ID: "Q1", Query: "什么是Dubbo？它有什么核心功能？", Category: "basic_concepts"},
		{ID: "Q4", Query: "服务调用失败可能有哪些原因？", Category: "troubleshooting"},
		{ID: "Q8", Query: "查询当前所有应用列表", Category: "tool_call"},
		{ID: "Q13", Query: "刚才说的那个配置项叫什么来着？", Category: "memory"},
		{ID: "Q21", Query: "帮我检查demo应用的健康状态，如果有问题告诉我原因", Category: "agent_reasoning"},
	}

	t.Log("========== AI Service API Integration Test ==========")
	t.Logf("Base URL: %s", baseURL)
	t.Logf("Test Cases: %d", len(testCases))

	for _, tc := range testCases {
		t.Run(tc.ID, func(t *testing.T) {
			startTime := time.Now()

			// Create unique session ID
			sessionID := fmt.Sprintf("test_%s_%d", tc.ID, time.Now().Unix())

			// Create session
			t.Logf("[%s] Creating session: %s", tc.ID, sessionID)
			sessionCreated := createSessionAPI(t, client, baseURL, sessionID)
			if !sessionCreated {
				t.Skipf("Skipping %s: service not available", tc.ID)
				return
			}

			time.Sleep(300 * time.Millisecond)

			// Send chat request
			t.Logf("[%s] Sending query: %s", tc.ID, tc.Query)
			resp, events, err := sendChatRequestAPI(t, client, baseURL, sessionID, tc.Query)
			duration := time.Since(startTime)

			if err != nil {
				t.Logf("[%s] ❌ Error: %v (Duration: %v)", tc.ID, err, duration)
				return
			}

			// Extract tool calls from events
			toolCalls := extractToolCalls(events)

			t.Logf("[%s] ✅ Success (Duration: %v)", tc.ID, duration)
			t.Logf("[%s] Response length: %d chars", tc.ID, len(resp))
			t.Logf("[%s] Tool calls: %v", tc.ID, toolCalls)
			t.Logf("[%s] Events received: %d", tc.ID, len(events))

			// Show first 200 chars of response
			preview := resp
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			t.Logf("[%s] Response preview: %s", tc.ID, preview)
		})
	}

	t.Log("==================================================")
}

// createSessionAPI creates a session via API
func createSessionAPI(t *testing.T, client *http.Client, baseURL, sessionID string) bool {
	reqBody := map[string]string{"session_id": sessionID}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", baseURL+"/api/v1/ai/sessions", bytes.NewReader(body))
	if err != nil {
		t.Logf("Failed to create request: %v", err)
		return false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Logf("Failed to connect to service: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Logf("Session creation failed: status %d, body: %s", resp.StatusCode, string(respBody))
		return false
	}

	t.Logf("Session created successfully")
	return true
}

// sendChatRequestAPI sends a chat request via API
func sendChatRequestAPI(t *testing.T, client *http.Client, baseURL, sessionID, query string) (string, []string, error) {
	reqBody := map[string]string{
		"message":   query,
		"sessionID": sessionID,
	}
	body, _ := json.Marshal(reqBody)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/ai/chat/stream", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	// Read SSE stream
	var events []string
	var responseBuilder strings.Builder

	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			events = append(events, chunk)
			responseBuilder.WriteString(chunk)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}

	return responseBuilder.String(), events, nil
}

// extractToolCalls extracts tool names from streaming events
func extractToolCalls(events []string) []string {
	var tools []string
	seen := make(map[string]bool)

	for _, event := range events {
		// Look for tool calling patterns
		if strings.Contains(event, "tool") && (strings.Contains(event, "calling") || strings.Contains(event, "call")) {
			// Try to extract tool name
			if idx := strings.Index(event, "tool="); idx > 0 {
				toolPart := event[idx:]
				if endIdx := strings.IndexAny(toolPart, ",}\n\""); endIdx > 0 {
					toolName := strings.TrimSpace(toolPart[5:endIdx])
					if !seen[toolName] && toolName != "" {
						seen[toolName] = true
						tools = append(tools, toolName)
					}
				}
			}
		}
	}

	return tools
}
