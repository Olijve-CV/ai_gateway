# 停止 Go AI Gateway 服务

停止在端口 8080 上运行的 Go AI Gateway 服务。

执行步骤：
1. 查找占用端口 8080 的进程
2. 终止该进程

Windows 命令：
```cmd
for /f "tokens=5" %a in ('netstat -ano ^| findstr :8080 ^| findstr LISTENING') do taskkill /PID %a /F
```

或者手动查找并终止：
```cmd
netstat -ano | findstr :8080
taskkill /PID <PID> /F
```
