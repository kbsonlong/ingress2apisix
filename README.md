# Ingress 到 APISIX 转换工具

这是一个将 Kubernetes Ingress 资源转换为 APISIX Ingress Controller 资源的工具。

## 功能特点

- 支持将 Kubernetes Ingress 资源转换为 APISIX Route 资源
- 保留原始 Ingress 的基本配置（主机名、路径、服务等）
- 支持输出为 JSON 格式的 APISIX 配置
- 支持批量转换集群中的所有 Ingress 资源

## 使用方法

1. 确保已安装 Go 1.16 或更高版本
2. 确保已配置好 Kubernetes 集群的访问凭证（kubeconfig）
3. 克隆本仓库
4. 运行以下命令：

```bash
go mod tidy
go run main.go
```

## 输出说明

转换后的 APISIX Route 资源将保存在 `output` 目录下，文件名格式为 `{namespace}-{ingress-name}.json`。

## 注意事项

- 目前仅支持基本的 Ingress 配置转换
- 部分高级特性（如注解转换）仍在开发中
- 建议在转换后检查生成的 APISIX 配置是否符合预期

## 开发计划

- [ ] 支持更多 Ingress 注解的转换
- [ ] 支持 TLS 配置的转换
- [ ] 支持直接应用到 APISIX 集群
- [ ] 添加命令行参数支持
- [ ] 添加转换验证功能