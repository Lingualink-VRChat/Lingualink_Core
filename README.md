# Lingualink Core

Lingualink Core 是一个开源的音频处理核心系统，专注于多语言音频转录和翻译功能。

## 🚀 特性

### 核心功能
- **多语言音频转录**：支持多种语言的音频转录
- **智能翻译**：将转录内容翻译成多种目标语言
- **灵活的提示词系统**：可自定义提示词模板
- **多LLM后端支持**：支持OpenAI、VLLM等多种LLM后端
- **负载均衡**：内置负载均衡器，支持多后端分发
- **智能响应解析**：自动解析LLM响应为结构化数据

### 技术特性
- **模块化架构**：清晰的模块划分，易于扩展
- **多种认证方式**：API Key、JWT、Webhook、匿名认证
- **RESTful API**：标准化的HTTP API接口
- **实时监控**：内置指标收集和监控
- **容器化部署**：完整的Docker支持
- **配置管理**：灵活的YAML配置系统

## 📦 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                      Lingualink Core                            │
├─────────────────┬─────────────────┬─────────────────────────────┤
│   HTTP API      │   认证系统      │      配置管理               │
├─────────────────┼─────────────────┼─────────────────────────────┤
│   音频处理器    │   提示词引擎    │      响应解析器             │
├─────────────────┼─────────────────┼─────────────────────────────┤
│   LLM管理器     │   负载均衡      │      指标收集               │
└─────────────────┴─────────────────┴─────────────────────────────┘
```

## 🛠️ 快速开始

### 环境要求
- Go 1.21+
- Docker & Docker Compose (可选)

### 本地开发

1. **克隆项目**
```bash
git clone https://github.com/Lingualink-VRChat/Lingualink_Core.git
cd Lingualink_Core
```

2. **快速启动**
```bash
# 检查环境
./start.sh --check

# 启动开发服务器
./start.sh --dev

# 或者使用开发辅助脚本
./dev.sh start
```

3. **配置环境**
```bash
# 编辑配置文件
vim config/config.yaml

# 设置环境变量（可选）
export VLLM_SERVER_URL=http://localhost:8000/v1
export MODEL_NAME=qwen2.5-32b-instruct
export API_KEY=your-api-key
```

4. **测试API**
```bash
# 快速测试
./quick_test.sh

# 完整API测试
./test_api.sh

# 音频处理测试
./test_audio.sh
```

### Docker 部署

```bash
# 使用开发脚本构建和运行
./dev.sh docker build
./dev.sh docker run

# 或者使用docker-compose
docker-compose up -d

# 仅启动 Lingualink Core
docker-compose up lingualink-core
```

## 🛠️ 开发工具

### 启动脚本

项目提供了便捷的启动脚本：

```bash
# 启动脚本
./start.sh --help          # 查看帮助
./start.sh --check         # 环境检查
./start.sh --dev           # 开发模式启动
./start.sh --build         # 构建后启动

# 开发辅助脚本
./dev.sh help              # 查看所有命令
./dev.sh start             # 启动开发服务器
./dev.sh build             # 构建应用
./dev.sh test              # 运行测试
./dev.sh test-api          # API测试
./dev.sh test-audio        # 音频测试
./dev.sh clean             # 清理构建文件
./dev.sh format            # 格式化代码
./dev.sh lint              # 代码检查
```

### 测试脚本

```bash
# 快速功能测试
./quick_test.sh

# 完整API测试套件
./test_api.sh

# 音频处理专项测试
./test_audio.sh
```

## 📚 API 文档

### 认证

支持多种认证方式：

#### API Key 认证
```bash
curl -H "X-API-Key: dev-key-123" http://localhost:8080/api/v1/process
```

#### Bearer Token 认证
```bash
curl -H "Authorization: Bearer your-jwt-token" http://localhost:8080/api/v1/process
```

### 主要接口

#### 健康检查
```bash
GET /api/v1/health
```

#### 音频处理
```bash
POST /api/v1/process
Content-Type: application/json

# 转录任务 (仅转录，不翻译)
{
  "audio": "base64-encoded-audio-data",
  "audio_format": "wav",
  "task": "transcribe"
}

# 翻译任务 (转录+翻译)
{
  "audio": "base64-encoded-audio-data",
  "audio_format": "wav",
  "task": "translate",
  "target_languages": ["en", "ja"]
}
```

#### 获取能力信息
```bash
GET /api/v1/capabilities
```

#### 支持的语言列表
```bash
GET /api/v1/languages
```

### 响应格式

**转录任务响应** (`task: "transcribe"`):
```json
{
  "request_id": "req_1234567890",
  "status": "success",
  "transcription": "原文转录内容",
  "translations": {},
  "processing_time": 1.5,
  "metadata": {
    "model": "qwen2.5-32b-instruct",
    "backend": "default"
  }
}
```

**翻译任务响应** (`task: "translate"`):
```json
{
  "request_id": "req_1234567890",
  "status": "success",
  "transcription": "原文转录内容",
  "translations": {
    "en": "English translation",
    "ja": "日本語翻訳"
  },
  "processing_time": 2.5,
  "metadata": {
    "model": "qwen2.5-32b-instruct",
    "backend": "default"
  }
}
```

## 🔧 配置说明

### 服务器配置
```yaml
server:
  mode: development  # development/production
  port: 8080
  host: 0.0.0.0
```

### 认证配置
```yaml
auth:
  strategies:
    - type: api_key
      enabled: true
      config:
        keys:
          dev-key-123:
            id: dev-user
            requests_per_minute: 100
```

### LLM 后端配置
```yaml
backends:
  load_balancer:
    strategy: round_robin
  providers:
    - name: default
      type: vllm
      url: http://localhost:8000/v1
      model: qwen2.5-32b-instruct
```

### 提示词配置
```yaml
prompt:
  defaults:
    task: translate
    target_languages: ["en", "ja", "zh"]
  languages:
    - code: zh
      names:
        display: "中文"
        english: "Chinese"
        native: "中文"
      aliases: ["chinese", "中文", "汉语", "zh-cn"]
```

## 🎯 支持的功能

### 音频格式
- WAV
- MP3
- M4A
- OPUS
- FLAC

### 语言支持
- 中文 (zh)
- 繁体中文 (zh-hant)
- 英文 (en)
- 日文 (ja)
- 韩文 (ko)
- 西班牙语 (es)
- 法语 (fr)
- 德语 (de)
- 俄语 (ru)
- 意大利语 (it)

### 任务类型
- `transcribe`: 仅转录，将音频内容转录成其原始语言的文本，不进行翻译
- `translate`: 转录+翻译，首先转录音频内容，然后翻译成指定的目标语言

### LLM 后端
- OpenAI Compatible API
- VLLM
- 可扩展支持其他后端

## 🛡️ 安全特性

- 多层认证机制
- API 密钥管理
- 请求限流
- 输入验证
- 日志记录
- 错误处理

## 📊 监控与指标

内置指标收集：
- HTTP 请求延迟
- 处理成功/失败计数
- 后端健康状态
- 资源使用情况

访问指标：
```bash
GET /api/v1/admin/metrics
```

## 🔄 开发工具

### CLI 工具
```bash
# 构建 CLI
go build -o lingualink cmd/cli/main.go

# 使用 CLI
./lingualink version
./lingualink server status
```

### 开发脚本
```bash
# 运行测试
go test ./...

# 代码格式化
go fmt ./...

# 构建
make build

# 启动开发环境
make dev
```

## 🤝 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 打开 Pull Request

## 📄 许可证

本项目采用 GPL-3.0 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

GPL-3.0 是一个强 copyleft 开源许可证，要求衍生作品也必须以相同许可证开源。这确保了项目及其衍生作品始终保持开源和自由。

## 🔗 相关链接

- [设计文档](docs/设计文档.md)
- [API 文档](docs/api.md)
- [部署指南](docs/deployment.md)
- [开发指南](docs/development.md)

## 📞 支持

如有问题或建议，请：
- 提交 Issue
- 加入讨论组
- 发送邮件到 support@lingualink.com

---

Made with ❤️ by the Lingualink Team 