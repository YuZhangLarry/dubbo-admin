# MCP Server 集成方案

## 1. 概述

本文档描述如何在 dubbo-admin 项目中集成 MCP (Model Context Protocol) Server，使 AI Agent 能够通过 MCP 协议调用 dubbo-admin 的只读查询功能。

### 1.1 目标

- 在 dubbo-admin 主服务中嵌入 MCP Server
- 暴露只读的 Dubbo 服务管理功能给 AI Agent
- ai/ 服务作为 MCP Client 调用 dubbo-admin 的能力

### 1.2 当前项目架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                         dubbo-admin (主服务)                         │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                   pkg/console/handler/                         │   │
│  │  - service.go      : 服务管理 (搜索、详情、配置)               │   │
│  │  - instance.go     : 实例管理                                 │   │
│  │  - application.go  : 应用管理                                 │   │
│  │  - overview.go     : 集群概览                                 │   │
│  │  - observability.go: 指标/追踪                                │   │
│  │  - search.go       : 全局搜索                                 │   │
│  │  - *_rule.go       : 条件路由/标签路由/配置规则                │   │
│  └───────────────────────────────────────────────────────────────┘   │
│                              │                                       │
│  ┌───────────────────────────┼───────────────────────────────────┐   │
│  │                           │ HTTP (Gin Router)                  │   │
│  │                           ↓                                   │   │
│  │  ┌─────────────────────────────────────────────────────────┐  │   │
│  │  │              [待实现] MCP Server 组件                     │  │   │
│  │  │  ┌───────────────────────────────────────────────────┐   │  │   │
│  │  │  │  方案A: HTTP Endpoint (推荐)                       │   │  │   │
│  │  │  │  - Endpoint: /api/v1/mcp                          │   │  │   │
│  │  │  │  - 使用 mark3labs/mcp-go SDK                      │   │  │   │
│  │  │  │  - 无状态 (Stateless)                             │   │  │   │
│  │  │  └───────────────────────────────────────────────────┘   │  │   │
│  │  │  ┌───────────────────────────────────────────────────┐   │  │   │
│  │  │  │  方案B: Stdio (本地开发)                          │   │  │   │
│  │  │  │  - ai/ 服务启动 dubbo-admin MCP Server            │   │  │   │
│  │  │  │  - 通过 stdio 通信                                │   │  │   │
│  │  │  └───────────────────────────────────────────────────┘   │  │   │
│  │  └─────────────────────────────────────────────────────────┘  │   │
│  └───────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              ↓
┌─────────────────────────────────────────────────────────────────────┐
│                            ai/ (AI 服务)                              │
│  ┌───────────────────────────────────────────────────────────────┐   │
│  │                    Runtime 组件系统                            │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐              │   │
│  │  │   Logger   │  │   Models   │  │   Memory   │              │   │
│  │  └────────────┘  └────────────┘  └────────────┘              │   │
│  │  ┌────────────┐  ┌────────────┐  ┌────────────┐              │   │
│  │  │    RAG     │  │   Server   │  │   Agent    │              │   │
│  │  └────────────┘  └────────────┘  └────────────┘              │   │
│  │  ┌──────────────────────────────────────────────────────────┐ │   │
│  │  │                    MCPToolManager                        │ │   │
│  │  │  - 当前: 使用 stdio 方式连接外部 MCP servers              │ │   │
│  │  │  - 计划: 支持 HTTP 方式连接 dubbo-admin MCP server        │ │   │
│  │  └──────────────────────────────────────────────────────────┘ │   │
│  └───────────────────────────────────────────────────────────────┘   │
│                                                                       │
│  当前 MCP 工具:                                                        │
│  - kubernetes: kubernetes-mcp-server (npx)                            │
│                                                                       │
│  待集成工具:                                                           │
│  - dubbo-admin: dubbo-admin 服务管理能力                              │
└─────────────────────────────────────────────────────────────────────┘
```

## 2. 当前项目结构

### 2.1 dubbo-admin 主服务

```
dubbo-admin/
├── pkg/
│   ├── console/
│   │   ├── handler/              # HTTP 处理器
│   │   │   ├── service.go        # 服务管理 (SearchServices, GetServiceDetail...)
│   │   │   ├── instance.go       # 实例管理 (SearchInstances, GetInstanceDetail...)
│   │   │   ├── application.go    # 应用管理
│   │   │   ├── overview.go       # 集群概览 (ClusterOverview, GetInstances...)
│   │   │   ├── observability.go  # 指标/追踪
│   │   │   ├── search.go         # 全局搜索
│   │   │   ├── configurator_rule.go   # 配置规则
│   │   │   ├── condition_rule.go      # 条件路由
│   │   │   ├── tag_rule.go            # 标签路由
│   │   │   └── ...
│   │   ├── context/             # 请求上下文
│   │   ├── model/               # 数据模型
│   │   └── service/             # 业务逻辑
│   │
│   └── config/
│       └── console/
│           └── config.go        # 配置结构 (Port, Auth, etc.)
```

### 2.2 ai/ 服务组件

```
ai/
├── runtime/
│   └── runtime.go               # 组件运行时 (Runtime, Component)
│
├── config/
│   ├── config.go                # 配置结构 (Type + Spec)
│   └── loader.go                # 配置加载器
│
├── component/
│   ├── logger/                  # 日志组件
│   ├── models/                  # AI 模型组件
│   ├── memory/                  # 记忆组件
│   ├── server/                  # HTTP 服务器组件
│   ├── rag/                     # RAG 组件 (支持 Milvus/Pinecone/Local)
│   │   ├── loaders/
│   │   ├── splitters/
│   │   ├── indexers/
│   │   ├── retrievers/
│   │   ├── rerankers/
│   │   ├── query/              # 查询处理层
│   │   └── mergers/            # 多路径合并
│   ├── agent/
│   │   └── react/              # ReAct Agent
│   └── tools/
│       ├── engine/
│       │   ├── tools.go        # 工具接口定义
│       │   ├── mock_tools.go   # Mock 工具
│       │   ├── memory_tools.go # 记忆工具
│       │   └── mcp_tools.go    # MCP 工具管理器 (使用 firebase/genkit/plugins/mcp)
│       ├── config.go           # ToolsConfig
│       └── tools.yaml          # 工具配置
│
├── main.go                     # 入口
└── config.yaml                 # 主配置
```

### 2.3 关键配置文件

**ai/config.yaml**
```yaml
project: dubbo-admin-ai
components:
  logger: component/logger/logger.yaml
  models: component/models/models.yaml
  server: component/server/server.yaml
  memory: component/memory/memory.yaml
  tools: component/tools/tools.yaml
  rag: component/rag/rag.yaml
  agent: component/agent/agent.yaml
```

**ai/component/tools/tools.yaml**
```yaml
type: tools
spec:
  enable_mock_tools: true
  enable_internal_tools: true
  enable_mcp_tools: false  # 测试环境禁用MCP工具
  mcp_host_name: "mcp_host"
  mcp_timeout: 30
  mcp_max_retries: 3
```

## 3. MCP Tools 定义

### 3.1 基于 Handler 映射的 Tool 列表

#### 服务管理 (service.go)

| Tool Name | 对应 Handler | 描述 | 参数 |
|-----------|-------------|------|------|
| `search_services` | SearchServices | 搜索 Dubbo 服务 | `filter`, `registry`, `pageSize`, `pageNumber` |
| `get_service_distribution` | GetServiceTabDistribution | 获取服务分布统计 | `registry`, `serviceName` |
| `get_service_detail` | GetServiceDetail | 获取服务详情 | `serviceName`, `group`, `version` |
| `get_service_interfaces` | GetServiceInterfaces | 获取服务接口列表 | `serviceName`, `group`, `version` |
| `get_service_timeout` | ServiceConfigTimeoutGET | 获取服务超时配置 | `serviceName`, `group`, `version` |
| `get_service_retry` | ServiceConfigRetryGET | 获取服务重试配置 | `serviceName`, `group`, `version` |
| `get_service_region_priority` | ServiceConfigRegionPriorityGET | 获取区域优先级 | `serviceName`, `group`, `version` |
| `get_service_argument_route` | ServiceConfigArgumentRouteGET | 获取参数路由 | `serviceName`, `group`, `version` |

#### 实例管理 (instance.go)

| Tool Name | 对应 Handler | 描述 | 参数 |
|-----------|-------------|------|------|
| `search_instances` | SearchInstances | 搜索服务实例 | `serviceName`, `appName`, `registry`, `pageSize`, `pageNumber` |
| `get_instance_detail` | GetInstanceDetail | 获取实例详情 | `address` (host:port) |

#### 应用管理 (application.go)

| Tool Name | 对应 Handler | 描述 | 参数 |
|-----------|-------------|------|------|
| `get_application_detail` | GetApplicationDetail | 获取应用详情 | `appName` |
| `get_application_instances` | GetApplicationTabInstanceInfo | 获取应用下的实例列表 | `appName` |
| `get_application_services` | GetApplicationServiceForm | 获取应用的服务列表 | `appName` |
| `search_applications` | ApplicationSearch | 搜索应用 | `filter`, `pageSize`, `pageNumber` |

#### 集群管理 (overview.go)

| Tool Name | 对应 Handler | 描述 | 参数 |
|-----------|-------------|------|------|
| `get_cluster_overview` | ClusterOverview | 获取集群统计概览 | 无 |
| `list_instances` | GetInstances | 列出所有实例 | 无 |
| `list_metadata` | GetMetas | 列出所有服务元数据 | 无 |
| `list_mappings` | GetMappings | 列出所有服务映射 | 无 |

#### 全局搜索 (search.go)

| Tool Name | 对应 Handler | 描述 | 参数 |
|-----------|-------------|------|------|
| `global_search` | BannerGlobalSearch | 全局搜索服务/应用/实例 | `keyword` |

### 3.2 工具分类汇总

| 分类 | 工具数量 |
|------|---------|
| 服务管理 | 8 |
| 实例管理 | 2 |
| 应用管理 | 4 |
| 集群管理 | 4 |
| 全局搜索 | 1 |
| **总计** | **19** |

### 3.3 Tool Schema 示例

```json
{
  "name": "search_services",
  "description": "搜索 Dubbo 服务，支持按服务名过滤和注册中心类型筛选",
  "inputSchema": {
    "type": "object",
    "properties": {
      "filter": {
        "type": "string",
        "description": "服务名过滤字符串，支持模糊匹配"
      },
      "registry": {
        "type": "string",
        "description": "注册中心类型，如: instance, zookeeper, nacos"
      },
      "pageSize": {
        "type": "integer",
        "description": "每页数量，默认 10",
        "default": 10
      },
      "pageNumber": {
        "type": "integer",
        "description": "页码，从 1 开始",
        "default": 1
      }
    }
  }
}
```

```json
{
  "name": "get_cluster_overview",
  "description": "获取 Dubbo 集群统计概览，包括应用数量、服务数量、实例数量等",
  "inputSchema": {
    "type": "object",
    "properties": {}
  }
}
```

## 4. 实现方案

### 4.1 方案 A: HTTP MCP Server (推荐)

在 dubbo-admin 主服务中集成 HTTP MCP Server。

#### 目录结构

```
dubbo-admin/
├── pkg/
│   ├── mcp/                           # MCP server 组件
│   │   ├── server.go                  # MCP Server 创建和启动
│   │   ├── tools.go                   # 工具注册
│   │   └── handlers/                  # 工具处理器
│   │       ├── service.go             # 服务相关
│   │       ├── instance.go            # 实例相关
│   │       ├── application.go         # 应用相关
│   │       ├── cluster.go             # 集群相关
│   │       └── search.go              # 搜索相关
│   │
│   └── config/
│       └── console/
│           └── config.go              # 添加 MCPConfig
│
└── main.go                            # 添加 MCP server 启动逻辑
```

#### 核心代码

**pkg/mcp/server.go**
```go
package mcp

import (
    "context"
    "log"

    "github.com/mark3labs/mcp-go/server"
    mcpserver "github.com/mark3labs/mcp-go/server"

    "github.com/apache/dubbo-admin/pkg/mcp/handlers"
    consolectx "github.com/apache/dubbo-admin/pkg/console/context"
)

// MCPServer wraps the MCP server with console context
type MCPServer struct {
    server   *mcpserver.MCPServer
    ctx      consolectx.Context
    address  string
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(ctx consolectx.Context, name, version string) *MCPServer {
    opts := []mcpserver.ServerOption{
        mcpserver.WithToolCapabilities(true),
    }
    s := mcpserver.NewMCPServer(name, version, opts...)

    // Register all tools
    handlers.RegisterTools(s, ctx)

    return &MCPServer{
        server:  s,
        ctx:     ctx,
    }
}

// Start starts the HTTP server
func (s *MCPServer) Start(address string) error {
    s.address = address
    log.Printf("Starting MCP server on %s", address)

    httpServer := mcpserver.NewStreamableHTTPServer(
        s.server,
        mcpserver.WithStateLess(true),
        mcpserver.WithEndpointPath("/api/v1/mcp"),
    )
    return httpServer.Start(address)
}
```

**pkg/mcp/handlers/service.go**
```go
package handlers

import (
    "context"
    "encoding/json"

    "github.com/mark3labs/mcp-go/mcp"

    "github.com/apache/dubbo-admin/pkg/console/handler"
    consolectx "github.com/apache/dubbo-admin/pkg/console/context"
)

var consoleCtx consolectx.Context

// RegisterTools registers all service-related tools
func RegisterTools(s *mcpserver.MCPServer, ctx consolectx.Context) {
    consoleCtx = ctx

    // search_services
    s.AddTool(mcp.NewTool("search_services",
        mcp.WithDescription("搜索 Dubbo 服务，支持按服务名过滤和注册中心类型筛选"),
        mcp.WithString("filter", mcp.Description("服务名过滤字符串，支持模糊匹配")),
        mcp.WithString("registry", mcp.Description("注册中心类型，如: instance, zookeeper, nacos")),
        mcp.WithNumber("pageSize", mcp.Description("每页数量"), mcp.DefaultNumber(10)),
        mcp.WithNumber("pageNumber", mcp.Description("页码， mcp.DefaultNumber(1)),
    ), searchServicesHandler)

    // get_service_detail
    s.AddTool(mcp.NewTool("get_service_detail",
        mcp.WithDescription("获取服务详情"),
        mcp.WithString("serviceName", mcp.Description("服务名称"), mcp.Required()),
        mcp.WithString("group", mcp.Description("服务组")),
        mcp.WithString("version", mcp.Description("服务版本")),
    ), getServiceDetailHandler)

    // ... other tools
}

func searchServicesHandler(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    args := req.GetArguments()

    // Build request from args
    filter := getStringArg(args, "filter", "")
    registry := getStringArg(args, "registry", "")
    pageSize := getIntArg(args, "pageSize", 10)
    pageNumber := getIntArg(args, "pageNumber", 1)

    // Call existing handler logic
    h := handler.SearchServices(consoleCtx)

    // Create gin context for handler (simulate HTTP request)
    // Note: This requires adapting the handler to work without gin.Context
    // or creating a service layer that both handlers and MCP can call

    result := map[string]any{
        "services": []any{},
        "total":    0,
    }

    content, _ := json.Marshal(result)

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            mcp.TextContent{Type: "text", Text: string(content)},
        },
    }, nil
}
```

### 4.2 方案 B: Stdio MCP Server (本地开发)

在 ai/ 服务中嵌入 dubbo-admin MCP Server，通过 stdio 通信。

#### 优点

- 无需额外 HTTP 端口
- 更简单的本地部署
- 与现有 kubernetes MCP server 模式一致

#### 实现方式

```
ai/
├── component/
│   └── tools/
│       ├── engine/
│       │   └── dubbo_admin_mcp.go    # Dubbo Admin MCP tool (stdio)
│       └── dubbo_admin_mcp_server/   # MCP server 实现
│           ├── main.go
│           ├── service.go
│           └── ...
```

**ai/component/tools/engine/dubbo_admin_mcp.go**
```go
package engine

import (
    "dubbo-admin-ai/runtime"
    "github.com/firebase/genkit/go/genkit"
    "github.com/firebase/genkit/go/plugins/mcp"
)

// NewDubboAdminMCPTools creates MCP tools for dubbo-admin
func NewDubboAdminMCPTools(rt *runtime.Runtime) (*MCPToolManager, error) {
    g := rt.GetGenkitRegistry()

    mcps := map[string][]string{
        "dubbo-admin": {
            "dubbo-admin-mcp-server",  // 自定义 MCP server 二进制
        },
    }

    host, err := DefineMCPHost(g, "dubbo_admin", mcps)
    if err != nil {
        return nil, err
    }

    activeTools, err := host.GetActiveTools(context.Background(), g)
    if err != nil {
        return nil, err
    }

    // Register tools...
    return &MCPToolManager{ /* ... */ }, nil
}
```

## 5. 配置设计

### 5.1 dubbo-admin 配置

在 `pkg/config/console/config.go` 中添加 MCPConfig：

```go
type Config struct {
    config.BaseConfig
    Port             int                    `json:"port" envconfig:"DUBBO_ADMIN_PORT"`
    MetricDashboards *MetricDashboardConfig `json:"metricDashboards"`
    TraceDashboards  *TraceDashboardConfig  `json:"traceDashboards"`
    Prometheus       string                 `json:"prometheus"`
    Grafana          string                 `json:"grafana"`
    Auth             *auth.Config           `json:"auth"`
    MCP              *MCPConfig             `json:"mcp"`           // 新增
}

type MCPConfig struct {
    Enable bool   `json:"enable" envconfig:"MCP_ENABLE"`
    Host   string `json:"host" envconfig:"MCP_HOST"`
    Port   int    `json:"port" envconfig:"MCP_PORT"`
}
```

### 5.2 ai/ 配置

**ai/component/tools/tools.yaml**
```yaml
type: tools
spec:
  enable_mock_tools: true
  enable_internal_tools: true
  enable_mcp_tools: true
  mcp_host_name: "mcp_host"
  mcp_timeout: 30
  mcp_max_retries: 3

  # 新增: dubbo-admin MCP server 配置
  dubbo_admin_mcp:
    enabled: true
    transport: "http"          # "stdio" 或 "http"
    url: "http://localhost:8888/api/v1/mcp"  # HTTP 模式
    # command: "dubbo-admin-mcp-server"       # Stdio 模式
    timeout: 30s
```

## 6. 传输层

### 6.1 HTTP 传输 (推荐)

```
客户端                            dubbo-admin
   │                                    │
   │  POST /api/v1/mcp                  │
   │  {                                 │
   │    "jsonrpc": "2.0",               │
   │    "id": 1,                        │
   │    "method": "tools/call",         │
   │    "params": {                     │
   │      "name": "search_services",    │
   │      "arguments": {...}            │
   │    }                               │
   │  }                                 │
   │ ──────────────────────────────────>│
   │                                    │
   │◄──────── 200 OK                    │
   │    {                               │
   │      "jsonrpc": "2.0",             │
   │      "id": 1,                      │
   │      "result": {                   │
   │        "content": [...]            │
   │      }                             │
   │    }                               │
```

### 6.2 Stdio 传输

ai/ 服务启动 dubbo-admin MCP Server 子进程，通过 stdin/stdout 通信。

## 7. 实现步骤

### 7.1 第一阶段：基础框架 (1-2 天)

1. **添加依赖**
   ```bash
   go get github.com/mark3labs/mcp-go@latest
   ```

2. **添加 MCP 配置**
   - 修改 `pkg/config/console/config.go`

3. **创建基础文件**
   - `pkg/mcp/server.go`
   - `pkg/mcp/tools.go`
   - `pkg/mcp/handlers/service.go`

4. **集成到主服务**
   - 在 `main.go` 中添加 MCP server 启动逻辑

### 7.2 第二阶段：工具实现 (3-5 天)

按优先级实现 handler：
- P0: `search_services`, `get_cluster_overview`, `list_instances`
- P1: `search_instances`, `get_service_detail`
- P2: 其他工具

**注意**: 需要重构现有 handler，将业务逻辑与 HTTP 层分离，使得 MCP 可以复用。

### 7.3 第三阶段：Client 集成 (2-3 天)

1. **修改 MCPToolManager**
   - 添加 HTTP transport 支持

2. **配置集成**
   - 添加 dubbo-admin MCP server URL 配置

### 7.4 第四阶段：测试 (2-3 天)

- 单元测试
- 集成测试
- Agent 对话测试

## 8. 架构改进建议

### 8.1 Handler 重构

当前 handler 直接依赖 `gin.Context`，建议重构：

```go
// 当前: handler 直接处理 HTTP
func SearchServices(ctx consolectx.Context) gin.HandlerFunc {
    return func(c *gin.Context) {
        req := model.NewServiceSearchReq()
        c.ShouldBindQuery(req)
        resp, err := service.GetSearchServices(ctx, req)
        c.JSON(http.StatusOK, model.NewSuccessResp(resp))
    }
}

// 建议: 分离业务逻辑
type ServiceService interface {
    Search(ctx context.Context, req *ServiceSearchReq) (*ServiceSearchResp, error)
}

// HTTP Handler
func (h *HttpHandler) SearchServices(c *gin.Context) {
    req := h.bindSearchRequest(c)
    resp, err := h.service.Search(c.Request.Context(), req)
    h.jsonResponse(c, resp, err)
}

// MCP Handler
func (h *MCPHandler) SearchServices(ctx context.Context, args map[string]any) (any, error) {
    req := h.bindSearchRequestFromArgs(args)
    return h.service.Search(ctx, req)
}
```

### 8.2 Service 层抽象

创建 `pkg/console/service/` 包，定义业务接口：

```go
package service

type ServiceService interface {
    Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
    GetDetail(ctx context.Context, req *DetailRequest) (*DetailResponse, error)
    GetDistribution(ctx context.Context, req *DistributionRequest) (*DistributionResponse, error)
}

type InstanceService interface {
    Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
    GetDetail(ctx context.Context, req *DetailRequest) (*DetailResponse, error)
}

type ApplicationService interface {
    GetDetail(ctx context.Context, appName string) (*ApplicationDetail, error)
    List(ctx context.Context, req *ListRequest) (*ListResponse, error)
}

type ClusterService interface {
    GetOverview(ctx context.Context) (*Overview, error)
    ListInstances(ctx context.Context) (*InstanceList, error)
    ListMetadata(ctx context.Context) (*MetadataList, error)
}
```

## 9. 参考资源

| 资源 | 链接 |
|------|------|
| MCP 官方规范 | https://modelcontextprotocol.io |
| Go SDK (mark3labs) | https://github.com/mark3labs/mcp-go |
| Genkit MCP Plugin | https://github.com/firebase/genkit |
| 社区服务器列表 | https://github.com/wong2/awesome-mcp-servers |

## 10. 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 3.0.0 | 2026-04-19 | 根据当前项目结构完善，添加 Genkit/MCPToolManager 说明 |
| 2.0.0 | 2026-04-14 | 简化设计，移除过度设计 |
| 1.0.0 | 2026-04-14 | 初始版本 |
