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

	consolectx "github.com/apache/dubbo-admin/pkg/console/context"
	"github.com/apache/dubbo-admin/pkg/mcp"
)

// RegisterSearchTools 注册搜索相关工具
func RegisterSearchTools(server *mcp.Server) {
	// global_search - 全局搜索
	server.RegisterTool(mcp.ToolDef{
		Name:        "global_search",
		Description: "全局搜索，支持搜索服务、实例、应用等资源",
		InputSchema: mcp.InputSchema{
			Type:     "object",
			Required: []string{"keyword"},
			Properties: map[string]mcp.PropertyDef{
				"keyword": {
					Type:        "string",
					Description: "搜索关键字",
				},
				"pageSize": {
					Type:        "integer",
					Description: "每页数量",
					Default:     10,
				},
				"pageNumber": {
					Type:        "integer",
					Description: "页码，从 1 开始",
					Default:     1,
				},
			},
		},
		Handler: GlobalSearch,
	})
}

// GlobalSearch 全局搜索
// 注意: 完整实现需要先修复项目中的 service 层依赖问题
func GlobalSearch(ctx consolectx.Context, args map[string]any) (*mcp.ToolResult, error) {
	keyword := getStringArg(args, "keyword", "")

	if keyword == "" {
		return errorResult(fmt.Errorf("keyword is required")), nil
	}

	pageSize := getIntArg(args, "pageSize", 10)
	pageNumber := getIntArg(args, "pageNumber", 1)

	result := map[string]any{
		"keyword":    keyword,
		"pageSize":   pageSize,
		"pageNumber": pageNumber,
		"results": map[string]any{
			"services":     []string{},
			"instances":    []string{},
			"applications": []string{},
		},
		"note": "Global search requires fixing pre-existing service layer dependencies",
	}

	return jsonResult(result)
}
