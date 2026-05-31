# Dubbo 常见问题排查

## 服务调用失败

### 症状：Consumer 无法调用 Provider 服务

**可能原因和排查步骤：**

1. **检查注册中心**
   - 确认 Zookeeper/Nacos 是否正常运行
   - 检查服务是否已成功注册
   - 使用 Dubbo Admin 查看服务列表

2. **检查网络连接**
   - 确认 Consumer 能访问 Provider 所在机器
   - 检查防火墙配置
   - 验证端口是否开放

3. **检查服务版本**
   - 确认 Consumer 和 Provider 的版本兼容
   - 检查序列化方式是否一致

4. **检查超时配置**
   - 增加超时时间：`<dubbo:reference timeout="5000"/>`
   - 检查是否有性能瓶颈

## 启动失败

### 症状：Provider 无法启动

**可能原因：**

1. **端口占用**
   ```
   检查 20880 端口是否被占用
   修改配置：<dubbo:protocol port="20881"/>
   ```

2. **配置错误**
   - 检查 XML 配置语法
   - 确认 bean ID 和 ref 一致

3. **依赖缺失**
   - 检查 Maven 依赖是否完整
   - 确认 Dubbo 版本兼容

## 性能问题

### 症状：服务调用缓慢

**排查方法：**

1. **开启监控**
   - 使用 Monitor 统计调用情况
   - 分析耗时分布

2. **检查序列化**
   - 优先使用 Hessian2 序列化
   - 避免大对象传输

3. **调整线程池**
   ```xml
   <dubbo:protocol threads="200"/>
   ```

## 查看日志

启用 Dubbo 日志：

```xml
<dubbo:protocol accesslog="/path/to/logs"/>
```

查看详细调用日志便于问题定位。
