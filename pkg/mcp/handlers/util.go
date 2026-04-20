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
	"encoding/json"
	"fmt"

	"github.com/apache/dubbo-admin/pkg/mcp"
)

// getStringArg 获取字符串参数
func getStringArg(args map[string]any, key, defaultValue string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return defaultValue
}

// getIntArg 获取整数参数
func getIntArg(args map[string]any, key string, defaultValue int) int {
	switch v := args[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return defaultValue
}

// getBoolArg 获取布尔参数
func getBoolArg(args map[string]any, key string, defaultValue bool) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return defaultValue
}

// errorResult 创建错误结果
func errorResult(err error) *mcp.ToolResult {
	return &mcp.ToolResult{
		Content: []mcp.Content{{
			Type: "text",
			Text: fmt.Sprintf("Error: %v", err),
		}},
		IsError: true,
	}
}

// textResult 创建文本结果
func textResult(text string) *mcp.ToolResult {
	return &mcp.ToolResult{
		Content: []mcp.Content{{
			Type: "text",
			Text: text,
		}},
	}
}

// jsonResult 创建 JSON 结果
func jsonResult(data any) (*mcp.ToolResult, error) {
	jsonData, err := formatJSON(data)
	if err != nil {
		return nil, err
	}
	return &mcp.ToolResult{
		Content: []mcp.Content{{
			Type: "text",
			Text: jsonData,
		}},
	}, nil
}

// formatJSON 格式化 JSON
func formatJSON(data any) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
