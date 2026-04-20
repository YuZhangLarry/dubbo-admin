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

package mcp

import (
	"encoding/json"
	"testing"

	consolectx "github.com/apache/dubbo-admin/pkg/console/context"
)

func TestServerInitialize(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: InitializeParams{
			ProtocolVersion: "2024-11-05",
			ClientInfo: ClientInfo{
				Name:    "test-client",
				Version: "1.0.0",
			},
		},
	}

	resp := server.handleRequest(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatal("result is not InitializeResult")
	}

	if result.ServerInfo.Name != "test-server" {
		t.Errorf("expected server name 'test-server', got '%s'", result.ServerInfo.Name)
	}

	if result.ServerInfo.Version != "1.0.0" {
		t.Errorf("expected server version '1.0.0', got '%s'", result.ServerInfo.Version)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version '2024-11-05', got '%s'", result.ProtocolVersion)
	}
}

func TestServerToolsList(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// 注册测试工具
	server.RegisterTool(ToolDef{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: InputSchema{
			Type: "object",
		},
		Handler: func(ctx consolectx.Context, args map[string]any) (*ToolResult, error) {
			return &ToolResult{
				Content: []Content{{Type: "text", Text: "test"}},
			}, nil
		},
	})

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/list",
	}

	resp := server.handleRequest(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(ToolListResult)
	if !ok {
		t.Fatal("result is not ToolListResult")
	}

	if len(result.Tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(result.Tools))
	}

	if result.Tools[0].Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got '%s'", result.Tools[0].Name)
	}
}

func TestServerToolsCall(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	// 注册测试工具
	server.RegisterTool(ToolDef{
		Name:        "echo_tool",
		Description: "Echo the input",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]PropertyDef{
				"message": {
					Type:        "string",
					Description: "Message to echo",
				},
			},
		},
		Handler: func(ctx consolectx.Context, args map[string]any) (*ToolResult, error) {
			msg, _ := args["message"].(string)
			return &ToolResult{
				Content: []Content{{Type: "text", Text: "Echo: " + msg}},
			}, nil
		},
	})

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      3,
		Method:  "tools/call",
		Params: map[string]any{
			"name": "echo_tool",
			"arguments": map[string]any{
				"message": "Hello, MCP!",
			},
		},
	}

	resp := server.handleRequest(req)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	result, ok := resp.Result.(CallToolResult)
	if !ok {
		t.Fatal("result is not CallToolResult")
	}

	if len(result.Content) != 1 {
		t.Errorf("expected 1 content item, got %d", len(result.Content))
	}

	if result.Content[0].Text != "Echo: Hello, MCP!" {
		t.Errorf("expected 'Echo: Hello, MCP!', got '%s'", result.Content[0].Text)
	}
}

func TestServerMethodNotFound(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      4,
		Method:  "unknown_method",
	}

	resp := server.handleRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestServerToolNotFound(t *testing.T) {
	server := NewServer("test-server", "1.0.0")

	req := &JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      5,
		Method:  "tools/call",
		Params: map[string]any{
			"name":      "non_existent_tool",
			"arguments": map[string]any{},
		},
	}

	resp := server.handleRequest(req)

	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}

	if resp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", resp.Error.Code)
	}
}

func TestJSONRPCRequest(t *testing.T) {
	jsonData := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "initialize",
		"params": {
			"protocolVersion": "2024-11-05",
			"capabilities": {},
			"clientInfo": {
				"name": "test-client",
				"version": "1.0.0"
			}
		}
	}`

	var req JSONRPCRequest
	err := json.Unmarshal([]byte(jsonData), &req)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got '%s'", req.JSONRPC)
	}

	if req.Method != "initialize" {
		t.Errorf("expected method 'initialize', got '%s'", req.Method)
	}
}

func TestJSONRPCResponse(t *testing.T) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      1,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    "test-server",
				Version: "1.0.0",
			},
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	var decoded JSONRPCResponse
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if decoded.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc '2.0', got '%s'", decoded.JSONRPC)
	}
}
