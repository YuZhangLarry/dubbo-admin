# Dubbo 注册中心

## 支持的注册中心

Dubbo 支持多种注册中心：

### Zookeeper

```xml
<dubbo:registry address="zookeeper://127.0.0.1:2181"/>
```

- 推荐的注册中心
- 支持集群部署
- 支持推送机制

### Nacos

```xml
<dubbo:registry address="nacos://127.0.0.1:8848"/>
```

- 阿里开源的动态服务发现
- 支持配置管理和服务发现
- 支持健康检查

### Redis

```xml
<dubbo:registry address="redis://127.0.0.1:6379"/>
```

- 基于 Redis 的注册中心
- 不推荐用于生产环境

### Multicast

```xml
<dubbo:registry address="multicast://224.5.6.7:1234"/>
```

- 基于广播的注册中心
- 仅用于开发测试环境

### Simple

```xml
<dubbo:registry address="simple://127.0.0.1:9090"/>
```

- 基于内存的注册中心
- 仅用于开发测试环境

## 注册中心对比

| 注册中心 | 推荐环境 | 集群支持 | 推送机制 |
|----------|----------|----------|----------|
| Zookeeper | 生产 | ✅ | ✅ |
| Nacos | 生产 | ✅ | ✅ |
| Redis | 不推荐 | ✅ | ❌ |
| Multicast | 测试 | ❌ | ✅ |
| Simple | 测试 | ❌ | ❌ |

## 配置建议

1. **生产环境**：使用 Zookeeper 或 Nacos
2. **开发环境**：可以使用 Multicast 或 Simple
3. **高可用要求**：部署注册中心集群
