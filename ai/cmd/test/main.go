/*
 * Enhanced API Integration Test for AI Service
 * Parses SSE stream and extracts actual responses
 * Run: go run api_test_full.go
 */

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	BaseURL = "http://127.0.0.1:8880"
)

type TestCase struct {
	ID       string
	Query    string
	Category string
}

type SessionResponse struct {
	Message string `json:"message"`
	Data    struct {
		SessionID string `json:"session_id"`
		Status    string `json:"status"`
	} `json:"data"`
}

type SSEMessage struct {
	Type string `json:"type"`
	Message struct {
		Content []ContentItem `json:"content"`
	} `json:"message"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text TextContent `json:"text"`
}

type TextContent struct {
	Value string `json:"value"`
	Annotations []interface{} `json:"annotations"`
}

var testCases = []TestCase{
	{ID: "Q1", Query: "什么是Dubbo？它有什么核心功能？", Category: "basic_concepts"},
	{ID: "Q4", Query: "服务调用失败可能有哪些原因？", Category: "troubleshooting"},
	{ID: "Q8", Query: "查询当前所有应用列表", Category: "tool_call"},
	{ID: "Q13", Query: "刚才说的那个配置项叫什么来着？", Category: "memory"},
	{ID: "Q21", Query: "帮我检查demo应用的健康状态，如果有问题告诉我原因", Category: "agent_reasoning"},
}

type TestResult struct {
	TestCase  TestCase
	Success   bool
	Duration  time.Duration
	Response  string
	ToolCalls []string
	Error     string
	Events    int
}

var results []TestResult

func main() {
	fmt.Println("========== AI Service API Integration Test ==========")
	fmt.Printf("Base URL: %s\n", BaseURL)
	fmt.Printf("Test Cases: %d\n\n", len(testCases))

	client := &http.Client{Timeout: 180 * time.Second}

	// Check if service is available
	if !checkService(client) {
		fmt.Println("❌ AI Service is not available. Please start the service first.")
		fmt.Println("   Run: cd ai && dubbo-admin-ai.exe")
		return
	}

	// Create a single session for all tests
	sessionID, err := createSession(client)
	if err != nil {
		fmt.Printf("❌ Failed to create session: %v\n", err)
		return
	}
	fmt.Printf("📝 Session ID: %s\n\n", sessionID)

	// Wait for session to be ready
	time.Sleep(1 * time.Second)

	for i, tc := range testCases {
		fmt.Printf("[%d/%d] Running %s (%s)\n", i+1, len(testCases), tc.ID, tc.Category)
		result := runTest(client, sessionID, tc)
		results = append(results, result)

		if result.Success {
			fmt.Printf("  ✅ Success - Duration: %v, Response: %d chars\n", result.Duration, len(result.Response))
		} else {
			fmt.Printf("  ❌ Failed: %v\n", result.Error)
		}

		if i < len(testCases)-1 {
			time.Sleep(1 * time.Second)
		}
		fmt.Println()
	}

	fmt.Println("==================================================")
	printSummary()
	printDetailedResponses()
}

func checkService(client *http.Client) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", BaseURL+"/api/v1/ai/sessions", nil)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return true
}

func createSession(client *http.Client) (string, error) {
	reqBody := map[string]string{"session_id": fmt.Sprintf("test_%d", time.Now().Unix())}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", BaseURL+"/api/v1/ai/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}

	var sessionResp SessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sessionResp); err != nil {
		return "", err
	}

	if sessionResp.Data.SessionID == "" {
		return "", fmt.Errorf("empty session_id in response")
	}

	return sessionResp.Data.SessionID, nil
}

func runTest(client *http.Client, sessionID string, tc TestCase) TestResult {
	startTime := time.Now()
	result := TestResult{
		TestCase: tc,
	}

	resp, events, err := sendChat(client, sessionID, tc.Query)
	result.Duration = time.Since(startTime)
	result.Events = len(events)

	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.Response = resp
	result.ToolCalls = extractToolCalls(events)
	result.Success = true

	return result
}

func sendChat(client *http.Client, sessionID, query string) (string, []string, error) {
	reqBody := map[string]string{
		"message":   query,
		"sessionID": sessionID,
	}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", BaseURL+"/api/v1/ai/chat/stream", bytes.NewReader(body))
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

	var events []string
	var responseText strings.Builder

	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			events = append(events, chunk)

			// Parse SSE events for text content
			// Format: event: content_block_delta
			//         data: {"delta":{"type":"text_delta","text":"..."},"index":10,"type":"content_block_delta"}
			lines := strings.Split(chunk, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "data:") {
					jsonStr := strings.TrimPrefix(line, "data:")
					var sseMsg map[string]interface{}
					if err := json.Unmarshal([]byte(jsonStr), &sseMsg); err == nil {
						// Check for content_block_delta with delta.text
						if msgType, ok := sseMsg["type"].(string); ok && msgType == "content_block_delta" {
							if delta, ok := sseMsg["delta"].(map[string]interface{}); ok {
								if deltaType, ok := delta["type"].(string); ok && deltaType == "text_delta" {
									if text, ok := delta["text"].(string); ok {
										responseText.WriteString(text)
									}
								}
							}
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			break
		}
	}

	return responseText.String(), events, nil
}

func extractToolCalls(events []string) []string {
	var tools []string
	seen := make(map[string]bool)

	for _, event := range events {
		if strings.Contains(event, "tool") && strings.Contains(event, "calling") {
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

func printSummary() {
	fmt.Println("\n========== 测试摘要 ==========")

	total := len(results)
	successCount := 0
	failCount := 0
	totalDuration := time.Duration(0)

	for _, r := range results {
		totalDuration += r.Duration
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Printf("总计: %d 测试\n", total)
	fmt.Printf("通过: %d\n", successCount)
	fmt.Printf("失败: %d\n", failCount)
	fmt.Printf("总耗时: %v\n", totalDuration)
	if total > 0 {
		fmt.Printf("平均耗时: %v\n", totalDuration/time.Duration(total))
	}
}

func printDetailedResponses() {
	fmt.Println("\n========== 详细响应 ==========")

	for _, r := range results {
		fmt.Printf("\n## %s: %s\n", r.TestCase.ID, r.TestCase.Query)
		fmt.Printf("分类: %s\n", r.TestCase.Category)
		fmt.Printf("状态: ")
		if r.Success {
			fmt.Println("✅ 成功")
		} else {
			fmt.Println("❌ 失败")
		}
		fmt.Printf("耗时: %v\n", r.Duration)
		fmt.Printf("事件数: %d\n", r.Events)
		if len(r.ToolCalls) > 0 {
			fmt.Printf("工具调用: %v\n", r.ToolCalls)
		}

		fmt.Printf("响应内容:\n%s\n", r.Response)
		fmt.Println(strings.Repeat("-", 60))
	}
}
