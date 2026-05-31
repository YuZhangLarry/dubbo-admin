# Dubbo 负载均衡策略

## 支持的负载均衡策略

Dubbo 内置了多种负载均衡策略：

### 1. Random（随机）

```java
<dubbo:reference loadbalance="random"/>
```

- 按权重设置随机概率
- 默认策略，适用于性能相当的服务实例

### 2. RoundRobin（轮询）

```java
<dubbo:reference loadbalance="roundrobin"/>
```

- 按公约后的权重设置轮询比率
- 适用于请求量均匀分配的场景

### 3. LeastActive（最少活跃调用）

```java
<dubbo:reference loadbalance="leastactive"/>
```

- 调用更活跃的实例接收更少的请求
- 适用于性能差异较大的服务实例

### 4. ConsistentHash（一致性哈希）

```java
<dubbo:reference loadbalance="consistenthash"/>
```

- 相同参数的请求总是发送到同一台机器
- 适用于有状态服务

### 5. ShortestResponse（最短响应）

```java
<dubbo:reference loadbalance="shortestresponse"/>
```

- 选择响应时间最短的实例
- 适用于对响应时间敏感的场景

## 选择建议

- **默认场景**：使用 random
- **性能不均**：使用 leastactive
- **需要粘性**：使用 consistenthash
- **低延迟要求**：使用 shortestresponse
