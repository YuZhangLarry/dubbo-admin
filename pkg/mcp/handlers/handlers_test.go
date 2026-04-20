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

package handlers

import (
	"fmt"
	"testing"

	"github.com/apache/dubbo-admin/pkg/mcp"
)

func TestRegisterServiceTools(t *testing.T) {
	server := mcp.NewServer("test-server", "1.0.0")

	RegisterServiceTools(server)

	tools := server.ListTools()

	// 验证服务工具已注册
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"search_services",
		"get_service_detail",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool '%s' to be registered", expected)
		}
	}
}

func TestRegisterClusterTools(t *testing.T) {
	server := mcp.NewServer("test-server", "1.0.0")

	RegisterClusterTools(server)

	tools := server.ListTools()

	// 验证集群工具已注册
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"get_cluster_info",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool '%s' to be registered", expected)
		}
	}
}

func TestRegisterSearchTools(t *testing.T) {
	server := mcp.NewServer("test-server", "1.0.0")

	RegisterSearchTools(server)

	tools := server.ListTools()

	// 验证搜索工具已注册
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"global_search",
	}

	for _, expected := range expectedTools {
		if !toolNames[expected] {
			t.Errorf("expected tool '%s' to be registered", expected)
		}
	}
}

func TestGetStringArg(t *testing.T) {
	args := map[string]any{
		"name": "test",
		"age":  25,
	}

	// 测试存在的字符串参数
	if result := getStringArg(args, "name", ""); result != "test" {
		t.Errorf("expected 'test', got '%s'", result)
	}

	// 测试不存在的参数，使用默认值
	if result := getStringArg(args, "missing", "default"); result != "default" {
		t.Errorf("expected 'default', got '%s'", result)
	}

	// 测试类型不匹配，使用默认值
	if result := getStringArg(args, "age", "default"); result != "default" {
		t.Errorf("expected 'default', got '%s'", result)
	}
}

func TestGetIntArg(t *testing.T) {
	args := map[string]any{
		"age":  25,
		"pi":   3.14,
		"name": "test",
	}

	// 测试存在的整数参数
	if result := getIntArg(args, "age", 0); result != 25 {
		t.Errorf("expected 25, got %d", result)
	}

	// 测试浮点数转换
	if result := getIntArg(args, "pi", 0); result != 3 {
		t.Errorf("expected 3, got %d", result)
	}

	// 测试不存在的参数，使用默认值
	if result := getIntArg(args, "missing", 10); result != 10 {
		t.Errorf("expected 10, got %d", result)
	}

	// 测试类型不匹配，使用默认值
	if result := getIntArg(args, "name", 10); result != 10 {
		t.Errorf("expected 10, got %d", result)
	}
}

func TestGetBoolArg(t *testing.T) {
	args := map[string]any{
		"active": true,
		"name":   "test",
	}

	// 测试存在的布尔参数
	if result := getBoolArg(args, "active", false); !result {
		t.Error("expected true, got false")
	}

	// 测试不存在的参数，使用默认值
	if result := getBoolArg(args, "missing", false); result {
		t.Error("expected false, got true")
	}

	// 测试类型不匹配，使用默认值
	if result := getBoolArg(args, "name", false); result {
		t.Error("expected false, got true")
	}
}

func TestErrorResult(t *testing.T) {
	err := fmt.Errorf("test error")
	result := errorResult(err)

	if !result.IsError {
		t.Error("expected IsError to be true")
	}

	if len(result.Content) != 1 {
		t.Errorf("expected 1 content item, got %d", len(result.Content))
	}

	if result.Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got '%s'", result.Content[0].Type)
	}
}

func TestTextResult(t *testing.T) {
	text := "test message"
	result := textResult(text)

	if result.IsError {
		t.Error("expected IsError to be false")
	}

	if len(result.Content) != 1 {
		t.Errorf("expected 1 content item, got %d", len(result.Content))
	}

	if result.Content[0].Text != text {
		t.Errorf("expected text '%s', got '%s'", text, result.Content[0].Text)
	}
}

func TestJsonResult(t *testing.T) {
	data := map[string]any{
		"key": "value",
		"num": 123,
	}

	result, err := jsonResult(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.IsError {
		t.Error("expected IsError to be false")
	}

	if len(result.Content) != 1 {
		t.Errorf("expected 1 content item, got %d", len(result.Content))
	}

	// 验证结果是有效的 JSON
	if result.Content[0].Type != "text" {
		t.Errorf("expected content type 'text', got '%s'", result.Content[0].Type)
	}
	// JSON 已被序列化为字符串，验证字符串包含预期内容
	text := result.Content[0].Text
	if text == "" {
		t.Error("expected non-empty JSON text")
	}
	// 验证字符串包含预期的 key
	if !contains(text, `"key"`) || !contains(text, `"value"`) {
		t.Errorf("expected JSON to contain key-value pair, got: %s", text)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
