# 重启 Go AI Gateway 服务

先停止再启动 Go AI Gateway 服务。

执行步骤：
1. 停止占用端口 8080 的进程
2. 等待 2 秒
3. 重新启动服务

Windows 命令：
1. 停止服务：
```cmd
for /f "tokens=5" %a in ('netstat -ano ^| findstr :8080 ^| findstr LISTENING') do taskkill /PID %a /F
```

2. 启动服务：
```bash
go run ./cmd/server/main.go
```
