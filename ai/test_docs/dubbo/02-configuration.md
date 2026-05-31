# Dubbo 配置指南

## Provider 配置

服务提供者配置示例（XML 方式）：

```xml
<!-- 服务提供方应用名，用于计算依赖关系 -->
<dubbo:application name="demo-provider"/>

<!-- 使用 Zookeeper 注册中心 -->
<dubbo:registry address="zookeeper://127.0.0.1:2181"/>

<!-- 用 dubbo 协议在 20880 端口暴露服务 -->
<dubbo:protocol name="dubbo" port="20880"/>

<!-- 声明需要暴露的服务接口 -->
<dubbo:service interface="com.example.DemoService" ref="demoService"/>

<!-- 和本地 bean 一样实现服务 -->
<bean id="demoService" class="com.example.impl.DemoServiceImpl"/>
```

## Consumer 配置

服务消费者配置示例：

```xml
<!-- 服务消费方应用名 -->
<dubbo:application name="demo-consumer"/>

<!-- 使用 Zookeeper 注册中心发现服务 -->
<dubbo:registry address="zookeeper://127.0.0.1:2181"/>

<!-- 生成远程服务代理 -->
<dubbo:reference id="demoService" interface="com.example.DemoService"/>
```

## 注解配置

使用注解简化配置：

```java
// 服务提供方
@Service(version = "1.0.0")
public class DemoServiceImpl implements DemoService {
    // ...
}

// 服务消费方
@Reference(version = "1.0.0")
private DemoService demoService;
```

## 配置项说明

| 配置项 | 说明 | 默认值 |
|--------|------|--------|
| protocol | 服务协议 | dubbo |
| port | 服务端口 | 20880 |
| timeout | 调用超时时间 | 1000ms |
| retries | 失败重试次数 | 2 |
| loadbalance | 负载均衡 | random |
