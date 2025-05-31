# Lingualink Core 测试指南

本文档介绍如何测试 Lingualink Core 的各项功能。

## 🚀 快速开始

### 1. 启动服务

首先启动 Lingualink Core 服务：

```bash
# 方式1: 直接运行
go run cmd/server/main.go

# 方式2: 使用Docker
docker-compose up lingualink-core

# 方式3: 构建后运行
go build -o lingualink cmd/server/main.go
./lingualink
```

服务启动后，默认监听在 `http://localhost:8080`

### 2. 基础功能测试

运行快速测试脚本验证基础功能：

```bash
./quick_test.sh
```

这个脚本会测试：
- ✅ 健康检查
- ✅ 系统能力查询
- ✅ 支持的语言列表
- ✅ 监控指标

## 🎵 音频处理测试

### 测试音频文件

项目的 `test/` 目录包含测试音频文件：
- `test.wav` (1.2MB) - WAV格式音频
- `test.opus` (26KB) - OPUS格式音频

### 运行音频测试

```bash
./test_audio.sh
```

这个脚本会测试：
- 🎯 WAV文件转录
- 🎯 OPUS文件转录  
- 🎯 转录+翻译（多语言）
- 🎯 JSON格式请求
- 🎯 不同任务类型（transcribe/translate/both）

## 🔍 全面API测试

运行完整的API测试套件：

```bash
./test_api.sh
```

这个脚本包含：

### 基础功能测试
- 健康检查
- 能力查询
- 语言列表
- 监控指标

### 认证与安全测试
- 有效API Key认证
- 无效API Key处理
- 无认证访问防护

### 音频处理测试
- 表单方式文件上传
- JSON方式Base64编码
- 多种音频格式支持
- 不同任务类型

### 错误处理测试
- 无效文件格式
- 缺少必需参数
- 错误的任务类型

### 性能测试
- 并发请求处理
- 响应时间统计

## 📊 测试输出示例

### 成功的音频处理响应

```json
{
  "request_id": "req_1701234567890",
  "status": "success",
  "transcription": "你好，这是一段测试音频。",
  "translations": {
    "英文": "Hello, this is a test audio.",
    "日文": "こんにちは、これはテストオーディオです。"
  },
  "processing_time": 2.5,
  "metadata": {
    "model": "qwen2.5-32b-instruct",
    "backend": "default",
    "audio_duration": 3.2,
    "audio_format": "wav"
  }
}
```

### 系统能力响应

```json
{
  "tasks": ["transcribe", "translate", "both"],
  "audio_formats": ["wav", "mp3", "m4a", "opus", "flac"],
  "languages": ["中文", "英文", "日文", "韩文", "西班牙语", "法语", "德语"],
  "backends": ["default"],
  "max_file_size": 33554432,
  "features": {
    "batch_processing": false,
    "real_time": false,
    "webhooks": true
  }
}
```

## 🛠️ 自定义测试

### 修改测试配置

可以通过环境变量或参数自定义测试：

```bash
# 自定义API地址
./test_api.sh --base-url http://localhost:8081

# 自定义API Key
./test_api.sh --api-key your-custom-key

# 使用环境变量
export BASE_URL="http://your-server:8080"
export API_KEY="your-api-key"
./quick_test.sh
```

### 添加自己的音频文件

将音频文件放到 `test/` 目录，然后修改脚本中的文件路径：

```bash
TEST_AUDIO_WAV="test/your-audio.wav"
TEST_AUDIO_OPUS="test/your-audio.opus"
```

## 🔧 故障排除

### 常见问题

1. **服务未启动**
   ```
   错误: 服务未运行
   解决: go run cmd/server/main.go
   ```

2. **认证失败**
   ```
   错误: HTTP 401 Unauthorized
   解决: 检查API Key配置
   ```

3. **音频处理失败**
   ```
   错误: LLM backend error
   解决: 检查LLM后端配置和状态
   ```

### 检查LLM后端

```bash
# 检查VLLM服务状态
curl $VLLM_SERVER_URL/v1/models

# 检查配置文件
cat config/config.yaml | grep -A 10 backends

# 查看服务日志
tail -f logs/app.log
```

### 调试模式

启用详细日志：

```bash
# 设置日志级别为debug
export LOG_LEVEL=debug
go run cmd/server/main.go
```

## 📈 测试报告

### 性能基准

正常情况下的预期性能：

- **健康检查**: < 10ms
- **能力查询**: < 50ms  
- **语言列表**: < 20ms
- **音频处理**: 依赖LLM后端（通常2-10秒）

### 支持的测试场景

- ✅ 本地开发环境
- ✅ Docker容器环境
- ✅ 生产环境验证
- ✅ CI/CD集成
- ✅ 负载测试

## 🚨 注意事项

1. **LLM后端依赖**: 音频处理功能需要配置有效的LLM后端
2. **文件大小限制**: 默认最大32MB音频文件
3. **网络超时**: 长音频处理可能需要较长时间
4. **并发限制**: 根据配置的限流设置调整并发测试

## 🤝 贡献测试用例

欢迎提交新的测试用例：

1. 在相应的脚本中添加测试函数
2. 确保测试覆盖边界情况
3. 添加适当的错误处理
4. 更新此文档说明

---

如有测试相关问题，请查看项目文档或提交Issue。 