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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dubbo-admin-ai/component/server"
	appruntime "dubbo-admin-ai/runtime"
)

// TestCase represents a single AI quality test case
type TestCase struct {
	ID          string   `json:"id"`
	Query       string   `json:"query"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	CheckPoints []string `json:"check_points"`
}

// TestResult represents the result of a single test case
type TestResult struct {
	TestCase         TestCase
	Success          bool
	Response         string
	Duration         time.Duration
	ToolCalls        []string
	StageTimings     map[string]time.Duration
	Error            string
	Score            Score
	StreamingEvents  []string
}

// Score represents the quality score of a test result
type Score struct {
	Accuracy    int `json:"accuracy"`    // 1-5
	Relevance   int `json:"relevance"`   // 1-5
	Completeness int `json:"completeness"` // 1-5
	Usability   int `json:"usability"`   // 1-5
	ToolUsed    bool `json:"tool_used"`
}

// TestSummary represents the summary of all test results
type TestSummary struct {
	TotalTests      int
	PassedTests     int
	FailedTests     int
	TotalDuration   time.Duration
	AvgResponseTime time.Duration
	CategoryStats   map[string]CategoryStats
	Results         []TestResult
}

// CategoryStats represents statistics for a test category
type CategoryStats struct {
	Total   int
	Passed  int
	Failed  int
	AvgTime time.Duration
}

// TestConfig represents the configuration for AI quality tests
type TestConfig struct {
	ServerBaseURL   string
	TestTimeout     time.Duration
	QuickTestOnly   bool
	Verbose         bool
	StopOnFirstFail bool
}

// Default test configuration
var defaultConfig = TestConfig{
	ServerBaseURL: "http://0.0.0.0:58880",
	TestTimeout:   120 * time.Second,
	QuickTestOnly: false,
	Verbose:       true,
	StopOnFirstFail: false,
}

// AI quality test cases
var testCases = []TestCase{
	// Dubbo特定场景测试
	{
		ID:          "Q1",
		Query:       "什么是Dubbo？它有什么核心功能？",
		Category:    "basic_concepts",
		Description: "基础概念类",
		CheckPoints: []string{
			"是否提到Dubbo是高性能、轻量级的Java RPC框架",
			"是否列出三大核心能力：远程调用、容错、服务发现",
			"回答是否清晰简洁",
		},
	},
	{
		ID:          "Q4",
		Query:       "服务调用失败可能有哪些原因？",
		Category:    "troubleshooting",
		Description: "故障排查类",
		CheckPoints: []string{
			"是否列出多种可能的原因",
			"是否给出对应的排查建议",
		},
	},
	{
		ID:          "Q8",
		Query:       "查询当前所有应用列表",
		Category:    "tool_call",
		Description: "工具调用类",
		CheckPoints: []string{
			"Agent是否正确识别需要调用工具",
			"是否调用了正确的MCP工具",
			"返回结果是否正确展示",
		},
	},
	{
		ID:          "Q13",
		Query:       "刚才说的那个配置项叫什么来着？",
		Category:    "memory",
		Description: "记忆能力类",
		CheckPoints: []string{
			"系统是否能从历史对话中找到上下文",
			"回答是否准确指回之前的内容",
		},
	},
	{
		ID:          "Q19",
		Query:       "查找关于服务发现的文档",
		Category:    "rag",
		Description: "RAG质量专项测试",
		CheckPoints: []string{
			"检索结果是否与'服务发现'高度相关",
			"检索召回数量是否合理",
		},
	},
	{
		ID:          "Q21",
		Query:       "帮我检查demo应用的健康状态，如果有问题告诉我原因",
		Category:    "agent_reasoning",
		Description: "Agent推理专项测试",
		CheckPoints: []string{
			"Agent是否分解为多个步骤",
			"工具调用顺序是否合理",
			"最终结论是否基于实际数据",
		},
	},
}

// Quick test cases for 10-minute test
var quickTestCases = []TestCase{
	{ID: "Q1", Query: "什么是Dubbo？它有什么核心功能？", Category: "basic_concepts"},
	{ID: "Q4", Query: "服务调用失败可能有哪些原因？", Category: "troubleshooting"},
	{ID: "Q8", Query: "查询当前所有应用列表", Category: "tool_call"},
	{ID: "Q13", Query: "刚才说的那个配置项叫什么来着？", Category: "memory"},
	{ID: "Q21", Query: "帮我检查demo应用的健康状态，如果有问题告诉我原因", Category: "agent_reasoning"},
}

// TestAIQuality tests the AI agent quality with various test cases
func TestAIQuality(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI quality test in short mode")
	}

	config := defaultConfig
	if os.Getenv("QUICK_TEST") == "true" {
		config.QuickTestOnly = true
	}

	// Setup test environment
	ctx := context.Background()
	configPath, _ := createTestConfigWithPort(t, 58880)
	rt, err := appruntime.Bootstrap(configPath, registerFactories)
	if err != nil {
		t.Fatalf("Failed to bootstrap runtime: %v", err)
	}
	defer rt.StopAll()

	// Get and start server
	serverComp, err := rt.GetComponent("server")
	if err != nil {
		t.Fatalf("Failed to get server component: %v", err)
	}

	svr, ok := serverComp.(*server.ServerComponent)
	if !ok {
		t.Fatalf("Server component is not the expected type")
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- svr.Start()
	}()
	defer func() {
		svr.Stop()
	}()

	// Wait for server to be ready
	time.Sleep(3 * time.Second)

	// Select test cases
	casesToTest := testCases
	if config.QuickTestOnly {
		casesToTest = quickTestCases
		t.Logf("Running quick test only (%d cases)", len(casesToTest))
	}

	// Run all test cases
	summary := runTestCases(t, ctx, config, casesToTest)

	// Print summary
	printTestSummary(t, summary)

	// Fail test if any test case failed
	if summary.FailedTests > 0 {
		t.Errorf("%d out of %d test cases failed", summary.FailedTests, summary.TotalTests)
	}
}

// runTestCases executes all test cases and returns summary
func runTestCases(t *testing.T, ctx context.Context, config TestConfig, cases []TestCase) TestSummary {
	summary := TestSummary{
		TotalTests:    len(cases),
		CategoryStats: make(map[string]CategoryStats),
		Results:       make([]TestResult, 0, len(cases)),
	}

	totalDuration := time.Duration(0)

	for i, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			result := runSingleTest(t, ctx, config, tc)
			summary.Results = append(summary.Results, result)

			totalDuration += result.Duration

			if result.Success {
				summary.PassedTests++
			} else {
				summary.FailedTests++
				if config.StopOnFirstFail {
					t.Fatalf("Test %s failed, stopping tests", tc.ID)
				}
			}

			// Update category stats
			stats := summary.CategoryStats[tc.Category]
			stats.Total++
			if result.Success {
				stats.Passed++
			} else {
				stats.Failed++
			}
			stats.AvgTime = (stats.AvgTime*time.Duration(stats.Total-1) + result.Duration) / time.Duration(stats.Total)
			summary.CategoryStats[tc.Category] = stats
		})

		// Small delay between tests
		if i < len(cases)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	summary.TotalDuration = totalDuration
	if summary.TotalTests > 0 {
		summary.AvgResponseTime = totalDuration / time.Duration(summary.TotalTests)
	}

	return summary
}

// runSingleTest executes a single test case
func runSingleTest(t *testing.T, ctx context.Context, config TestConfig, tc TestCase) TestResult {
	startTime := time.Now()
	result := TestResult{
		TestCase:     tc,
		StageTimings: make(map[string]time.Duration),
	}

	// Create session for this test
	sessionID := fmt.Sprintf("test_%s_%d", tc.ID, time.Now().Unix())

	t.Logf("Running test %s: %s", tc.ID, tc.Query)

	// Create session
	createSessionResp, err := createSession(ctx, config.ServerBaseURL, sessionID)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create session: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	if config.Verbose {
		t.Logf("Session created: %s", createSessionResp)
	}

	// Wait for session to be persisted
	time.Sleep(200 * time.Millisecond)

	// Send chat request
	chatResp, events, err := sendChatRequest(ctx, config.ServerBaseURL, sessionID, tc.Query, config.TestTimeout)
	if err != nil {
		result.Error = fmt.Sprintf("Chat request failed: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	result.Response = chatResp
	result.StreamingEvents = events
	result.Duration = time.Since(startTime)

	// Parse streaming events to extract tool calls and stage timings
	analyzeStreamingEvents(events, &result)

	// Evaluate result
	result.Success = evaluateResult(result)
	result.Score = calculateScore(result)

	if config.Verbose {
		t.Logf("Test completed in %v", result.Duration)
		t.Logf("Response length: %d chars", len(result.Response))
		t.Logf("Tool calls: %v", result.ToolCalls)
		if !result.Success {
			t.Logf("Test may have issues: %s", result.Error)
		}
	}

	return result
}

// createSession creates a new session
func createSession(ctx context.Context, baseURL, sessionID string) (string, error) {
	reqBody := map[string]string{"session_id": sessionID}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/ai/sessions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Message string `json:"message"`
		Data    struct {
			SessionID string `json:"session_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Data.SessionID, nil
}

// sendChatRequest sends a chat request and returns response and streaming events
func sendChatRequest(ctx context.Context, baseURL, sessionID, query string, timeout time.Duration) (string, []string, error) {
	reqBody := map[string]string{
		"message":   query,
		"sessionID": sessionID,
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/v1/ai/chat/stream", bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: timeout}
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

// analyzeStreamingEvents extracts information from streaming events
func analyzeStreamingEvents(events []string, result *TestResult) {
	// Look for stage names and tool calls in events
	for _, event := range events {
		// Extract stage names
		if strings.Contains(event, "stage") {
			// Parse stage timing if available
		}

		// Extract tool calls
		if strings.Contains(event, "tool") && strings.Contains(event, "calling") {
			// Parse tool name
			if idx := strings.Index(event, "tool="); idx > 0 {
				toolPart := event[idx:]
				if endIdx := strings.IndexAny(toolPart, ",}\n\""); endIdx > 0 {
					toolName := strings.TrimSpace(toolPart[5:endIdx])
					result.ToolCalls = append(result.ToolCalls, toolName)
				}
			}
		}
	}

	// Deduplicate tool calls
	seen := make(map[string]bool)
	uniqueTools := make([]string, 0, len(result.ToolCalls))
	for _, tool := range result.ToolCalls {
		if !seen[tool] {
			seen[tool] = true
			uniqueTools = append(uniqueTools, tool)
		}
	}
	result.ToolCalls = uniqueTools
}

// evaluateResult evaluates if the test result is successful
func evaluateResult(result TestResult) bool {
	// Basic checks
	if result.Error != "" {
		return false
	}

	if result.Response == "" {
		result.Error = "Empty response"
		return false
	}

	// Check for timeout
	if result.Duration > 60*time.Second {
		result.Error = fmt.Sprintf("Response took too long: %v", result.Duration)
		return false
	}

	// Check for streaming
	if len(result.StreamingEvents) == 0 {
		result.Error = "No streaming events received"
		return false
	}

	return true
}

// calculateScore calculates the quality score for a test result
func calculateScore(result TestResult) Score {
	score := Score{
		Accuracy:    3, // Default middle score
		Relevance:   3,
		Completeness: 3,
		Usability:   3,
		ToolUsed:    len(result.ToolCalls) > 0,
	}

	// Adjust scores based on result characteristics
	if result.Success {
		score.Accuracy = 4
		score.Relevance = 4
	}

	if len(result.Response) > 100 {
		score.Completeness = 4
	}

	if result.Duration < 30*time.Second {
		score.Usability = 5
	} else if result.Duration < 60*time.Second {
		score.Usability = 4
	} else {
		score.Usability = 3
	}

	return score
}

// printTestSummary prints the test summary
func printTestSummary(t *testing.T, summary TestSummary) {
	t.Log("\n========== AI Quality Test Summary ==========")
	t.Logf("Total Tests: %d", summary.TotalTests)
	t.Logf("Passed: %d", summary.PassedTests)
	t.Logf("Failed: %d", summary.FailedTests)
	t.Logf("Total Duration: %v", summary.TotalDuration)
	t.Logf("Avg Response Time: %v", summary.AvgResponseTime)

	t.Log("\n---------- Category Statistics ----------")
	for category, stats := range summary.CategoryStats {
		t.Logf("%s: %d/%d passed, avg time: %v",
			category, stats.Passed, stats.Total, stats.AvgTime)
	}

	t.Log("\n---------- Individual Results ----------")
	for _, result := range summary.Results {
		status := "PASS"
		if !result.Success {
			status = "FAIL"
		}
		t.Logf("[%s] %s (%s) - %v - score: %+v",
			result.TestCase.ID, status, result.TestCase.Category,
			result.Duration, result.Score)
		if !result.Success && result.Error != "" {
			t.Logf("    Error: %s", result.Error)
		}
	}
	t.Log("==============================================\n")
}

// TestAIQualityQuickTest runs only the quick test cases
func TestAIQualityQuickTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping AI quality test in short mode")
	}

	// Set environment variable for quick test
	os.Setenv("QUICK_TEST", "true")
	defer os.Unsetenv("QUICK_TEST")

	TestAIQuality(t)
}

// createTestConfigWithPort creates a temporary config with absolute paths and custom port for testing
func createTestConfigWithPort(t *testing.T, port int) (configPath string, cleanup func()) {
	_, file, _, _ := runtime.Caller(0)
	aiDir := filepath.Dir(filepath.Dir(filepath.Dir(file)))
	tmpDir := t.TempDir()

	// Helper to convert path to forward slashes for YAML
	toSlash := func(p string) string {
		return strings.ReplaceAll(p, "\\", "/")
	}

	// Create a modified agent.yaml with absolute prompt path
	agentConfigPath := filepath.Join(tmpDir, "agent.yaml")
	agentContent, _ := os.ReadFile(filepath.Join(aiDir, "component", "agent", "agent.yaml"))
	modifiedAgentContent := string(agentContent)
	modifiedAgentContent = strings.Replace(modifiedAgentContent, `prompt_base_path: "./prompts"`,
		fmt.Sprintf(`prompt_base_path: "%s"`, toSlash(filepath.Join(aiDir, "prompts"))), 1)
	_ = os.WriteFile(agentConfigPath, []byte(modifiedAgentContent), 0644)

	// Create a modified server.yaml with custom port
	serverConfigPath := filepath.Join(tmpDir, "server.yaml")
	serverContent := fmt.Sprintf(`type: server
spec:
  port: %d # Server port
  host: "0.0.0.0" # Server host
  debug: false # Debug mode
  cors_origins: ["*"] # CORS origins
  read_timeout: 30 # Read timeout in seconds
  write_timeout: 30 # Write timeout in seconds
`, port)
	_ = os.WriteFile(serverConfigPath, []byte(serverContent), 0644)

	// Create config file with absolute paths
	configContent := fmt.Sprintf(`project: dubbo-admin-ai
version: 1.0.0
components:
  logger: %s
  memory: %s
  models: %s
  server: %s
  tools: %s
  rag: %s
  agent: %s
`,
		toSlash(filepath.Join(aiDir, "component", "logger", "logger.yaml")),
		toSlash(filepath.Join(aiDir, "component", "memory", "memory.yaml")),
		toSlash(filepath.Join(aiDir, "component", "models", "models.yaml")),
		toSlash(serverConfigPath),
		toSlash(filepath.Join(aiDir, "component", "tools", "tools.yaml")),
		toSlash(filepath.Join(aiDir, "component", "rag", "rag.yaml")),
		toSlash(agentConfigPath),
	)

	configPath = filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(configPath, []byte(configContent), 0644)

	cleanup = func() {} // tmpDir will be cleaned up by t.TempDir()
	return configPath, cleanup
}
