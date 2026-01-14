# 启动 Go AI Gateway 服务

在 `go` 目录下运行服务，端口为 8080。

执行步骤：
1. 切换到 go 目录
2. 使用 `go run ./cmd/server/main.go` 启动服务
3. 服务在后台运行

```bash
cd go && go run ./cmd/server/main.go
```

注意：如果端口 8080 已被占用，请先使用 `/stop-go` 停止服务。
