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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	consolectx "github.com/apache/dubbo-admin/pkg/console/context"
	"github.com/gin-gonic/gin"
)

// Server MCP 服务器
type Server struct {
	name           string
	version        string
	tools          map[string]ToolDef
	consoleContext consolectx.Context
}

// NewServer 创建 MCP 服务器
func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
		tools:   make(map[string]ToolDef),
	}
}

// SetConsoleContext 设置 console context
func (s *Server) SetConsoleContext(ctx consolectx.Context) {
	s.consoleContext = ctx
}

// GetConsoleContext 获取 console context
func (s *Server) GetConsoleContext() consolectx.Context {
	return s.consoleContext
}

// RegisterTool 注册工具
func (s *Server) RegisterTool(tool ToolDef) {
	// 设置 server 引用，让 handler 可以访问 context
	tool.Server = s
	s.tools[tool.Name] = tool
}

// ListTools 列出所有工具
func (s *Server) ListTools() []ToolDef {
	result := make([]ToolDef, 0, len(s.tools))
	for _, tool := range s.tools {
		result = append(result, tool)
	}
	return result
}

// HandleHTTP 处理 HTTP 请求
func (s *Server) HandleHTTP(c *gin.Context) {
	// 读取请求体
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
		})
		return
	}

	// 解析 JSON-RPC 请求
	var req JSONRPCRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, JSONRPCResponse{
			JSONRPC: "2.0",
			Error: &JSONRPCError{
				Code:    -32700,
				Message: "Parse error",
			},
		})
		return
	}

	// 路由处理
	resp := s.handleRequest(&req)

	c.JSON(http.StatusOK, resp)
}

// handleRequest 处理 JSON-RPC 请求
func (s *Server) handleRequest(req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(req)
	default:
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}
}

// handleInitialize 处理 initialize 请求
func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo: ServerInfo{
				Name:    s.name,
				Version: s.version,
			},
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{},
			},
		},
	}
}

// handleToolsList 处理 tools/list 请求
func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	tools := make([]Tool, 0, len(s.tools))
	for _, def := range s.tools {
		tools = append(tools, Tool{
			Name:        def.Name,
			Description: def.Description,
			InputSchema: def.InputSchema,
		})
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: ToolListResult{
			Tools: tools,
		},
	}
}

// handleToolsCall 处理 tools/call 请求
func (s *Server) handleToolsCall(req *JSONRPCRequest) *JSONRPCResponse {
	params, ok := req.Params.(map[string]any)
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Invalid params",
			},
		}
	}

	name, ok := params["name"].(string)
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32602,
				Message: "Tool name is required",
			},
		}
	}

	tool, ok := s.tools[name]
	if !ok {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Tool not found: " + name,
			},
		}
	}

	// 获取参数
	arguments, _ := params["arguments"].(map[string]any)

	// 调用工具处理器，传入 console context
	result, err := tool.Handler(s.consoleContext, arguments)
	if err != nil {
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: CallToolResult{
				Content: []Content{{
					Type: "text",
					Text: err.Error(),
				}},
				IsError: true,
			},
		}
	}

	// 转换 ToolResult 到 CallToolResult
	callResult := CallToolResult{
		Content: make([]Content, len(result.Content)),
		IsError: result.IsError,
	}
	for i, c := range result.Content {
		callResult.Content[i] = Content{
			Type: c.Type,
			Text: c.Text,
		}
	}

	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  callResult,
	}
}
