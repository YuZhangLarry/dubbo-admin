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

// RegisterServiceTools 注册服务相关工具
func RegisterServiceTools(server *mcp.Server) {
	// search_services - 搜索服务
	server.RegisterTool(mcp.ToolDef{
		Name:        "search_services",
		Description: "搜索 Dubbo 服务，支持按服务名过滤和分页",
		InputSchema: mcp.InputSchema{
			Type: "object",
			Properties: map[string]mcp.PropertyDef{
				"keywords": {
					Type:        "string",
					Description: "服务名搜索关键字，支持模糊匹配",
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
		Handler: SearchServices,
	})

	// get_service_detail - 获取服务详情
	server.RegisterTool(mcp.ToolDef{

		Name:        "get_service_detail",
		Description: "获取服务详情，包括服务分布和实例信息",
		InputSchema: mcp.InputSchema{
			Type:     "object",
			Required: []string{"serviceName"},
			Properties: map[string]mcp.PropertyDef{
				"serviceName": {
					Type:        "string",
					Description: "服务名称",
				},
				"side": {
					Type:        "string",
					Description: "服务端或消费者 (provider/consumer)",
					Default:     "provider",
				},
			},
		},
		Handler: GetServiceDetail,
	})
}

// SearchServices 搜索服务
// 注意: 完整实现需要先修复项目中的 service 层依赖问题
func SearchServices(ctx consolectx.Context, args map[string]any) (*mcp.ToolResult, error) {
	keywords := getStringArg(args, "keywords", "")
	pageSize := getIntArg(args, "pageSize", 10)
	pageNumber := getIntArg(args, "pageNumber", 1)

	result := map[string]any{
		"keywords":   keywords,
		"pageSize":   pageSize,
		"pageNumber": pageNumber,
		"services":   []string{},
		"totalCount": 0,
		"note":       "Service search requires fixing pre-existing service layer dependencies",
	}

	return jsonResult(result)
}

// GetServiceDetail 获取服务详情
// 注意: 完整实现需要先修复项目中的 service 层依赖问题
func GetServiceDetail(ctx consolectx.Context, args map[string]any) (*mcp.ToolResult, error) {
	serviceName := getStringArg(args, "serviceName", "")

	if serviceName == "" {
		return errorResult(fmt.Errorf("serviceName is required")), nil
	}

	side := getStringArg(args, "side", "provider")

	result := map[string]any{
		"serviceName": serviceName,
		"side":        side,
		"instances":   []string{},
		"note":        "Service detail requires fixing pre-existing service layer dependencies",
	}

	return jsonResult(result)
}
