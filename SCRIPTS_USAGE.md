# Lingualink Core 脚本使用指南

## 📋 概览

项目包含两个主要脚本：
- `start.sh` - 启动脚本，带智能端口冲突检测
- `stop.sh` - 停止脚本，优雅停止服务和清理资源

## 🚀 启动脚本 (start.sh)

### 基本用法

```bash
# 开发模式启动（默认）
./start.sh

# 构建模式启动
./start.sh --build

# 详细输出模式
./start.sh --dev --verbose

# 仅检查环境
./start.sh --check

# 显示运行时信息
./start.sh --info
```

### 端口冲突自动处理

启动脚本会自动检查端口8080占用情况：

1. **手动模式（默认）**：检测到冲突时会提示选择
   ```bash
   ./start.sh
   # 如果端口被占用，会显示：
   # 1) 自动停止冲突的进程
   # 2) 手动处理后重新启动  
   # 3) 退出脚本
   ```

2. **自动模式**：设置环境变量自动停止冲突进程
   ```bash
   AUTO_STOP_CONFLICTING=true ./start.sh
   ```

### 环境变量配置

```bash
# 基本配置
export VLLM_SERVER_URL="http://localhost:8000/v1"
export MODEL_NAME="qwen2.5-32b-instruct"  
export API_KEY="your-api-key"
export LOG_LEVEL="debug"

# 自动处理端口冲突
export AUTO_STOP_CONFLICTING="true"

# 然后启动
./start.sh --build
```

### 输出和日志

- 所有输出同时显示在终端并保存到 `logs/server_YYYYMMDD_HHMMSS.log`
- 启动信息包括：运行时参数、环境变量、配置预览
- 使用 `--verbose` 可查看Go编译过程等详细信息

## 🛑 停止脚本 (stop.sh)

### 基本用法

```bash
# 查看当前运行状态
./stop.sh --status

# 优雅停止所有服务
./stop.sh --stop

# 强制停止所有服务
./stop.sh --stop --force

# 释放特定端口
./stop.sh --port 8080

# 停止服务并清理文件
./stop.sh --all

# 仅清理临时文件
./stop.sh --cleanup
```

### 端口管理

```bash
# 释放8080端口（优雅停止）
./stop.sh --port 8080

# 强制释放8080端口
./stop.sh --port 8080 --force

# 释放其他端口
./stop.sh --port 3000 --force
```

### 完整清理

```bash
# 停止所有服务 + 清理文件
./stop.sh --all

# 强制停止所有服务 + 清理文件
./stop.sh --all --force
```

## 🔧 常见使用场景

### 场景1：正常开发流程

```bash
# 1. 启动开发服务
./start.sh --dev --verbose

# 2. 开发完成后停止
./stop.sh --stop
```

### 场景2：端口被占用错误

当遇到 `bind: address already in use` 错误时：

```bash
# 方法1：使用停止脚本释放端口
./stop.sh --port 8080
./start.sh

# 方法2：查看端口占用情况
./stop.sh --status

# 方法3：强制清理所有相关进程
./stop.sh --all --force
./start.sh
```

### 场景3：自动化部署

```bash
# 设置自动停止冲突进程
export AUTO_STOP_CONFLICTING=true

# 停止现有服务
./stop.sh --all --force

# 启动新服务
./start.sh --build
```

### 场景4：生产环境重启

```bash
# 1. 优雅停止现有服务
./stop.sh --stop

# 2. 清理临时文件
./stop.sh --cleanup

# 3. 构建并启动新版本
./start.sh --build
```

## 📊 状态监控

### 查看详细状态
```bash
./stop.sh --status
```

输出包括：
- 运行中的二进制服务进程
- 运行中的Go开发进程  
- 端口8080占用情况
- 进程ID和命令详情

### 查看运行时信息
```bash
./start.sh --info
```

输出包括：
- 当前目录和脚本参数
- Go版本和模块信息
- 所有环境变量
- 配置文件预览

## ⚠️ 注意事项

1. **优雅停止 vs 强制停止**
   - 优雅停止：先发送SIGTERM，等待2秒，再发送SIGKILL
   - 强制停止：直接发送SIGKILL，可能导致数据丢失
   - 建议先尝试优雅停止，失败后再强制停止

2. **端口冲突处理**
   - 脚本只会自动停止Lingualink相关进程
   - 其他进程占用端口需要手动处理
   - 可使用 `lsof -i :8080` 查看端口占用详情

3. **日志管理**
   - 日志文件保存在 `logs/` 目录
   - 停止脚本可以清理7天以上的旧日志
   - 使用 `--verbose` 模式获取更详细的调试信息

4. **权限要求**
   - 脚本需要可执行权限：`chmod +x start.sh stop.sh`
   - 停止其他用户进程可能需要sudo权限

## 🐛 故障排除

### 问题1：端口仍然被占用
```bash
# 查看详细端口信息
lsof -i :8080

# 强制释放端口
./stop.sh --port 8080 --force

# 查看所有Lingualink进程
ps aux | grep lingualink
```

### 问题2：脚本无权限
```bash
# 添加执行权限
chmod +x start.sh stop.sh

# 检查权限
ls -la *.sh
```

### 问题3：lsof命令未找到
```bash
# macOS
brew install lsof

# Ubuntu/Debian
sudo apt-get install lsof

# CentOS/RHEL
sudo yum install lsof
```

### 问题4：进程停不掉
```bash
# 查看进程树
pstree | grep lingualink

# 强制杀死所有相关进程
./stop.sh --all --force

# 手动杀死顽固进程
sudo kill -9 $(pgrep -f lingualink)
```

## 📝 总结

这两个脚本提供了完整的服务生命周期管理：

- **智能启动**：自动检测环境、处理端口冲突、详细日志
- **优雅停止**：分级停止策略、资源清理、状态监控  
- **开发友好**：详细输出、调试信息、错误处理
- **生产就绪**：自动化支持、日志管理、故障恢复

使用这些脚本可以有效避免 "地址已被占用" 等常见问题，提升开发和部署效率。 