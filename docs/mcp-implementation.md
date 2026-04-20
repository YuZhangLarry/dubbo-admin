# MCP Server 技术实现方案

## 1. 设计原则

### 1.1 核心约束

- **不引入外部 MCP SDK**：MCP 协议本质是 JSON-RPC 2.0，直接实现
- **复用现有架构**：基于现有的 Gin Router 和 Handler
- **最小侵入**：不修改现有业务逻辑
- **只读操作**：所有 MCP 工具都是查询，无副作用

### 1.2 技术选型

| 层级 | 技术方案 |
|------|---------|
| 协议层 | JSON-RPC 2.0 (手动实现) |
| HTTP层 | Gin (现有) |
| 业务层 | 复用 `pkg/console/service/` |
| 序列化 | 标准库 `encoding/json` |

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         dubbo-admin 主服务                           │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Gin Router                                │    │
│  │  ┌──────────────────┐  ┌─────────────────────────────────┐  │    │
│  │  │  现有 API 路由    │  │        MCP 路由 (新增)           │  │    │
│  │  │  /api/*          │  │        /api/v1/mcp              │  │    │
│  │  └──────────────────┘  │                                 │  │    │
│  │                        │  POST ──> MCPHandler            │  │    │
│  │                        │                                 │  │    │
│  │                        └─────────────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                       │
│                              ↓                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    MCP Layer (新增)                           │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │    │
│  │  │   types.go   │  │  server.go   │  │  tools.go    │      │    │
│  │  │  (协议定义)   │  │ (请求处理)   │  │ (工具注册)   │      │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘      │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                       │
│                              ↓                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                   Tool Handlers (新增)                        │    │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │    │
│  │  │ service.go   │  │ cluster.go   │  │  search.go   │      │    │
│  │  └──────────────┘  └──────────────┘  └──────────────┘      │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                       │
│                              ↓                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                   现有 Service 层                             │    │
│  │            pkg/console/service/*.go (复用)                    │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
dubbo-admin/
├── pkg/
│   ├── mcp/                           # MCP 协议层 (新增)
│   │   ├── types.go                   # 协议类型定义
│   │   ├── server.go                  # MCP Server 实现
│   │   ├── tools.go                   # 工具注册表
│   │   └── handlers/                  # 工具处理器 (新增)
│   │       ├── service.go             # 服务相关工具
│   │       ├── instance.go            # 实例相关工具
│   │       ├── application.go         # 应用相关工具
│   │       ├── cluster.go             # 集群相关工具
│   │       └── search.go              # 搜索相关工具
│   │
│   ├── config/
│   │   └── console/
│   │       └── config.go              # 添加 MCPConfig
│   │
│   ├── console/
│   │   ├── handler/                   # 现有 HTTP Handler (不变)
│   │   ├── service/                   # 现有 Service (复用)
│   │   └── router/
│   │       └── router.go              # 添加 MCP 路由
│   │
│   └── ...
│
├── main.go                            # 添加 MCP 初始化
```

## 3. MCP 协议实现

### 3.1 JSON-RPC 2.0 核心

MCP 基于 JSON-RPC 2.0，核心消息格式：

```go
// pkg/mcp/types.go

// JSONRPCRequest JSON-RPC 2.0 请求
type JSONRPCRequest struct {
    JSONRPC string      `json:"jsonrpc"` // 必须是 "2.0"
    ID      interface{} `json:"id"`       // 请求 ID
    Method  string      `json:"method"`   // 方法名
    Params  interface{} `json:"params"`   // 参数
}

// JSONRPCResponse JSON-RPC 2.0 响应
type JSONRPCResponse struct {
    JSONRPC string        `json:"jsonrpc"`
    ID      interface{}   `json:"id"`
    Result  interface{}   `json:"result,omitempty"`
    Error   *JSONRPCError `json:"error,omitempty"`
}

// JSONRPCError JSON-RPC 2.0 错误
type JSONRPCError struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

### 3.2 MCP 方法路由

| 方法 | 说明 |
|------|------|
| `initialize` | 初始化连接，返回服务器能力 |
| `tools/list` | 列出所有可用工具 |
| `tools/call` | 调用指定工具 |

## 4. 工具实现

### 4.1 工具注册表

```go
// pkg/mcp/tools.go

// ToolDef 工具定义
type ToolDef struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    InputSchema InputSchema `json:"inputSchema"`
    Handler     ToolHandler `json:"-"`
}

// InputSchema 输入参数 schema
type InputSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]PropertyDef `json:"properties,omitempty"`
    Required   []string               `json:"required,omitempty"`
}

// PropertyDef 属性定义
type PropertyDef struct {
    Type        string   `json:"type"`
    Description string   `json:"description,omitempty"`
    Default     any      `json:"default,omitempty"`
    Enum        []string `json:"enum,omitempty"`
}

// ToolHandler 工具处理器类型
type ToolHandler func(ctx context.Context, args map[string]any) (*ToolResult, error)

// ToolResult 工具执行结果
type ToolResult struct {
    Content []Content `json:"content"`
    IsError bool      `json:"isError,omitempty"`
}

// Content 内容块
type Content struct {
    Type string `json:"type"` // "text"
    Text string `json:"text"`
}
```

### 4.2 服务管理工具

**pkg/mcp/handlers/service.go**

```go
package handlers

import (
    "context"
    "encoding/json"

    "github.com/apache/dubbo-admin/pkg/mcp"
    "github.com/apache/dubbo-admin/pkg/console/service"
)

// RegisterServiceTools 注册服务相关工具
func RegisterServiceTools(server *mcp.Server) {
    // search_services
    server.RegisterTool(mcp.ToolDef{
        Name:        "search_services",
        Description: "搜索 Dubbo 服务，支持按服务名过滤和注册中心类型筛选",
        InputSchema: mcp.InputSchema{
            Type: "object",
            Properties: map[string]mcp.PropertyDef{
                "filter": {
                    Type:        "string",
                    Description: "服务名过滤字符串，支持模糊匹配",
                },
                "registry": {
                    Type:        "string",
                    Description: "注册中心类型，如: instance, zookeeper, nacos",
                    Enum:        []string{"instance", "zookeeper", "nacos", "consul"},
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

    // get_service_detail
    server.RegisterTool(mcp.ToolDef{
        Name:        "get_service_detail",
        Description: "获取服务详情",
        InputSchema: mcp.InputSchema{
            Type:     "object",
            Required: []string{"serviceName"},
            Properties: map[string]mcp.PropertyDef{
                "serviceName": {
                    Type:        "string",
                    Description: "服务名称",
                },
                "group": {
                    Type:        "string",
                    Description: "服务组",
                },
                "version": {
                    Type:        "string",
                    Description: "服务版本",
                },
            },
        },
        Handler: GetServiceDetail,
    })
}

// SearchServices 搜索服务
func SearchServices(ctx context.Context, args map[string]any) (*mcp.ToolResult, error) {
    filter := getStringArg(args, "filter", "")
    registry := getStringArg(args, "registry", "")
    pageSize := getIntArg(args, "pageSize", 10)
    pageNumber := getIntArg(args, "pageNumber", 1)

    // 调用 service 层
    req := &model.ServiceSearchReq{
        Filter:    filter,
        Registry:  registry,
        PageSize:  pageSize,
        PageNumber: pageNumber,
    }

    resp, err := service.GetSearchServices(ctx, req)
    if err != nil {
        return &mcp.ToolResult{
            Content: []mcp.Content{{
                Type: "text",
                Text: fmt.Sprintf("Error: %v", err),
            }},
            IsError: true,
        }, nil
    }

    data, _ := json.Marshal(resp)
    return &mcp.ToolResult{
        Content: []mcp.Content{{
            Type: "text",
            Text: string(data),
        }},
    }, nil
}
```

### 4.3 集群管理工具

**pkg/mcp/handlers/cluster.go**

```go
package handlers

import (
    "context"
    "encoding/json"

    "github.com/apache/dubbo-admin/pkg/mcp"
    "github.com/apache/dubbo-admin/pkg/console/service"
)

// RegisterClusterTools 注册集群相关工具
func RegisterClusterTools(server *mcp.Server) {
    // get_cluster_overview
    server.RegisterTool(mcp.ToolDef{
        Name:        "get_cluster_overview",
        Description: "获取 Dubbo 集群统计概览，包括应用数量、服务数量、实例数量等",
        InputSchema: mcp.InputSchema{
            Type: "object",
        },
        Handler: GetClusterOverview,
    })

    // list_instances
    server.RegisterTool(mcp.ToolDef{
        Name:        "list_instances",
        Description: "列出所有实例",
        InputSchema: mcp.InputSchema{
            Type: "object",
        },
        Handler: ListInstances,
    })
}

// GetClusterOverview 获取集群概览
func GetClusterOverview(ctx context.Context, args map[string]any) (*mcp.ToolResult, error) {
    resp, err := service.GetClusterOverview(ctx)
    if err != nil {
        return errorResult(err), nil
    }

    data, _ := json.Marshal(resp)
    return &mcp.ToolResult{
        Content: []mcp.Content{{
            Type: "text",
            Text: string(data),
        }},
    }, nil
}
```

## 5. HTTP 集成

### 5.1 MCP Handler

```go
// pkg/mcp/server.go

package mcp

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"

    "github.com/gin-gonic/gin"
)

// Server MCP 服务器
type Server struct {
    name    string
    version string
    tools   map[string]ToolDef
}

// NewServer 创建 MCP 服务器
func NewServer(name, version string) *Server {
    return &Server{
        name:    name,
        version: version,
        tools:   make(map[string]ToolDef),
    }
}

// RegisterTool 注册工具
func (s *Server) RegisterTool(tool ToolDef) {
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
                Message: "Method not found",
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

    // 调用工具处理器
    result, err := tool.Handler(context.Background(), arguments)
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

    return &JSONRPCResponse{
        JSONRPC: "2.0",
        ID:      req.ID,
        Result:  result,
    }
}
```

### 5.2 路由注册

```go
// pkg/console/router/router.go (修改)

import (
    "github.com/apache/dubbo-admin/pkg/mcp"
    "github.com/apache/dubbo-admin/pkg/mcp/handlers"
)

func InitRouter(r *gin.Engine, ctx consolectx.Context) {
    // ... 现有路由 ...

    // 初始化 MCP Server (如果启用)
    if config.MCPEnabled() {
        mcpServer := mcp.NewServer("dubbo-admin", "1.0.0")

        // 注册所有工具
        handlers.RegisterServiceTools(mcpServer)
        handlers.RegisterInstanceTools(mcpServer)
        handlers.RegisterApplicationTools(mcpServer)
        handlers.RegisterClusterTools(mcpServer)
        handlers.RegisterSearchTools(mcpServer)

        // 注册 MCP 路由
        r.POST("/api/v1/mcp", mcpServer.HandleHTTP)
    }
}
```

## 6. 配置

### 6.1 配置结构

```go
// pkg/config/console/config.go (添加)

type Config struct {
    config.BaseConfig
    Port             int                    `json:"port" envconfig:"DUBBO_ADMIN_PORT"`
    // ... 现有字段 ...
    MCP              *MCPConfig             `json:"mcp"`           // 新增
}

// MCPConfig MCP 服务器配置
type MCPConfig struct {
    Enable bool   `json:"enable" envconfig:"MCP_ENABLE"`
    Path   string `json:"path" envconfig:"MCP_PATH"`  // 默认: /api/v1/mcp
}
```

### 6.2 配置文件

```yaml
# conf/config.yaml
admin:
  port: 8888
  mcp:
    enable: true
    path: /api/v1/mcp
```

## 7. 工具列表

### 7.1 P0 (核心工具)

| 工具名 | 描述 | 复用方法 |
|--------|------|---------|
| `search_services` | 搜索服务 | `service.GetSearchServices` |
| `get_cluster_overview` | 集群概览 | `service.GetClusterOverview` |
| `list_instances` | 列出实例 | `manager.List(DataplaneResourceList)` |
| `global_search` | 全局搜索 | `handler.BannerGlobalSearch` |

### 7.2 P1 (重要工具)

| 工具名 | 描述 | 复用方法 |
|--------|------|---------|
| `get_service_detail` | 服务详情 | `service.GetServiceDetail` |
| `search_instances` | 搜索实例 | `service.SearchInstances` |
| `get_application_detail` | 应用详情 | `service.GetApplicationDetail` |

### 7.3 P2 (辅助工具)

| 工具名 | 描述 |
|--------|------|
| `get_service_distribution` | 服务分布 |
| `list_metadata` | 元数据列表 |
| `list_mappings` | 映射列表 |

## 8. 实现步骤

### 第一阶段：基础框架 (1 天)

1. 创建 `pkg/mcp/` 目录
2. 实现 `types.go` - 协议类型定义
3. 实现 `server.go` - MCP Server 核心
4. 添加配置支持

### 第二阶段：核心工具 (2 天)

1. 实现 `handlers/service.go` - 服务管理工具
2. 实现 `handlers/cluster.go` - 集群管理工具
3. 实现 `handlers/search.go` - 搜索工具

### 第三阶段：路由集成 (0.5 天)

1. 修改 `router.go` 添加 MCP 路由
2. 修改 `main.go` 添加初始化逻辑

### 第四阶段：测试 (1 天)

1. 单元测试
2. 集成测试
3. AI 服务联调测试

## 9. 测试方案

### 9.1 手动测试

使用 curl 测试 MCP 端点：

```bash
# 初始化
curl -X POST http://localhost:8888/api/v1/mcp \
  -H "Content-Type: application/json" \
  -d '{
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
  }'

# 列出工具
curl -X POST http://localhost:8888/api/v1/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'

# 调用工具
curl -X POST http://localhost:8888/api/v1/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "get_cluster_overview",
      "arguments": {}
    }
  }'
```

### 9.2 AI 服务集成测试

修改 ai/ 服务的 MCPToolManager，添加 HTTP 传输支持：

```go
// ai/component/tools/engine/mcp_tools.go (修改)

type HTTPMCPClient struct {
    baseURL string
    client  *http.Client
}

func (c *HTTPMCPClient) ListTools() ([]Tool, error) {
    // HTTP POST /api/v1/mcp
    // method: tools/list
}

func (c *HTTPMCPClient) CallTool(name string, args map[string]any) (any, error) {
    // HTTP POST /api/v1/mcp
    // method: tools/call
}
```

## 10. 辅助函数

```go
// pkg/mcp/handlers/util.go

package handlers

import "fmt"

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
```

## 11. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0.0 | 2026-04-19 | 初始版本，不使用外部 SDK 的实现方案 |
