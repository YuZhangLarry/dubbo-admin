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
	consolectx "github.com/apache/dubbo-admin/pkg/console/context"
	"github.com/apache/dubbo-admin/pkg/mcp"
)

// RegisterClusterTools 注册集群相关工具
func RegisterClusterTools(server *mcp.Server) {
	// get_cluster_info - 获取集群基本信息
	server.RegisterTool(mcp.ToolDef{
		Name:        "get_cluster_info",
		Description: "获取 Dubbo 集群基本信息",
		InputSchema: mcp.InputSchema{
			Type: "object",
		},
		Handler: GetClusterInfo,
	})
}

// GetClusterInfo 获取集群基本信息
// 注意: 此实现返回基本信息。完整的集群统计需要先修复项目中的 service 层依赖问题
func GetClusterInfo(ctx consolectx.Context, args map[string]any) (*mcp.ToolResult, error) {
	// 返回基本信息
	info := map[string]any{
		"name":    "dubbo-cluster",
		"version": "1.0.0",
		"status":  "running",
		"note":    "Full cluster overview requires fixing pre-existing service layer dependencies",
	}

	return jsonResult(info)
}
