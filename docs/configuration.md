# Lingualink Core 配置指南

## 概述

Lingualink Core 使用 YAML 配置文件进行配置管理，支持环境变量覆盖。配置文件默认位于 `config/config.yaml`。

**配置优先级**: 环境变量 > config.yaml > 默认值

---

## 快速开始

1. 复制配置模板：
```bash
cp config/config.template.yaml config/config.yaml
cp config/api_keys.template.json config/api_keys.json
```

2. 编辑配置文件：
```bash
vim config/config.yaml
```

3. 启动服务：
```bash
./start_local.sh
```

---

## 配置结构

### 服务器配置 (server)

```yaml
server:
  mode: development    # development 或 production
  port: 8080           # HTTP 服务端口
  host: 0.0.0.0        # 监听地址
```

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `mode` | string | `development` | 运行模式，影响日志级别和调试信息 |
| `port` | int | `8080` | HTTP 服务监听端口 |
| `host` | string | `0.0.0.0` | 监听地址 |

---

### 认证配置 (auth)

```yaml
auth:
  strategies:
    - type: api_key
      enabled: true
      config:
        keys_file: config/api_keys.json
```

#### API Key 认证

API Key 从 JSON 文件加载，文件格式：

```json
{
  "keys": {
    "lingualink-demo-key": {
      "id": "default-user",
      "requests_per_minute": -1
    },
    "production-key-xxx": {
      "id": "prod-user",
      "requests_per_minute": 100
    }
  }
}
```

| 字段 | 说明 |
|-----|------|
| `id` | 用户标识符 |
| `requests_per_minute` | 每分钟请求限制，-1 表示无限制 |

---

### LLM 后端配置 (backends)

```yaml
backends:
  load_balancer:
    strategy: round_robin    # 负载均衡策略
  
  providers:
    - name: default
      type: vllm             # 后端类型: vllm, openai
      url: http://localhost:8000/v1
      model: qwenOmni7
      api_key: "your-api-key"
      parameters:
        temperature: 0.7
        max_tokens: 1000
        top_p: 0.95
```

#### 后端提供者配置

| 字段 | 类型 | 必须 | 说明 |
|-----|------|-----|------|
| `name` | string | **是** | 后端名称（唯一标识）|
| `type` | string | **是** | 后端类型：`vllm`, `openai` |
| `url` | string | **是** | API 端点 URL |
| `model` | string | **是** | 模型名称 |
| `api_key` | string | 否 | API 密钥（如果后端需要）|
| `parameters` | object | 否 | LLM 参数配置 |

#### 多后端配置示例

```yaml
backends:
  load_balancer:
    strategy: round_robin
  
  providers:
    - name: primary
      type: vllm
      url: http://gpu-server-1:8000/v1
      model: qwen2.5-32b-instruct
      parameters:
        temperature: 0.3
    
    - name: secondary
      type: vllm
      url: http://gpu-server-2:8000/v1
      model: qwen2.5-32b-instruct
      parameters:
        temperature: 0.3
```

---

### LLM 参数配置

详细的 LLM 参数配置说明见 [llm-parameters.md](./llm-parameters.md)。

#### 基础参数

| 参数 | 类型 | 范围 | 说明 |
|-----|------|-----|------|
| `temperature` | float | 0.0-2.0 | 输出随机性，0.0 完全确定性 |
| `max_tokens` | int | - | 最大输出 token 数 |
| `top_p` | float | 0.0-1.0 | 核采样参数 |

#### 高级参数

| 参数 | 类型 | 说明 |
|-----|------|------|
| `top_k` | int | Top-K 采样（VLLM 支持）|
| `repetition_penalty` | float | 重复惩罚（开源模型）|
| `frequency_penalty` | float | 频率惩罚（OpenAI）|
| `presence_penalty` | float | 存在惩罚（OpenAI）|
| `stop` | string[] | 停止词列表 |
| `seed` | int | 随机种子 |

#### 推荐配置

**精确翻译**（推荐）:
```yaml
parameters:
  temperature: 0.2
  max_tokens: 120
  top_p: 0.95
```

**创意写作**:
```yaml
parameters:
  temperature: 1.0
  max_tokens: 2000
  top_p: 0.95
```

---

### 提示词配置 (prompt)

```yaml
prompt:
  language_management_strategy: merge    # merge 或 override
  
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

#### 语言管理策略

| 策略 | 说明 |
|-----|------|
| `merge` | 与 `languages.default.yaml` 合并 |
| `override` | 完全覆盖默认语言配置 |

#### 语言配置字段

| 字段 | 说明 |
|-----|------|
| `code` | 语言代码（ISO 639-1）|
| `names.display` | 显示名称 |
| `names.english` | 英文名称 |
| `names.native` | 本地名称 |
| `aliases` | 别名列表 |

---

### 音频处理配置 (audio)

```yaml
audio:
  max_size: 33554432           # 最大文件大小（字节），32MB
  supported_formats:
    - wav
    - mp3
    - m4a
    - opus
    - flac
  conversion:
    enabled: true              # 启用 FFmpeg 转换
    target_format: wav         # 目标格式
    sample_rate: 16000         # 采样率
```

---

## 环境变量

可以使用环境变量覆盖配置文件中的值：

| 环境变量 | 对应配置 |
|---------|---------|
| `SERVER_PORT` | `server.port` |
| `SERVER_MODE` | `server.mode` |
| `VLLM_SERVER_URL` | `backends.providers[0].url` |
| `MODEL_NAME` | `backends.providers[0].model` |
| `API_KEY` | `backends.providers[0].api_key` |

**使用示例**:
```bash
export SERVER_PORT=8100
export VLLM_SERVER_URL=http://localhost:8000/v1
./start_local.sh
```

---

## Docker 配置

### docker-compose.yml

```yaml
services:
  lingualink-core:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config:ro
      - ./logs:/app/logs
    environment:
      - SERVER_MODE=production
```

### 挂载配置文件

```bash
docker run -v $(pwd)/config:/app/config lingualink-core
```

---

## 配置验证

启动时服务会验证配置：

```bash
# 检查配置
./start_local.sh

# 查看加载的配置
curl http://localhost:8080/api/v1/capabilities
```

**常见错误**:

| 错误 | 原因 |
|-----|------|
| `config file not found` | 配置文件不存在 |
| `invalid backend url` | 后端 URL 格式错误 |
| `no api keys configured` | API 密钥文件为空 |

---

## 生产环境建议

1. **使用强 API Key**:
   ```json
   {
     "keys": {
       "prod-xxx-secure-key": {
         "id": "production-user",
         "requests_per_minute": 100
       }
     }
   }
   ```

2. **启用生产模式**:
   ```yaml
   server:
     mode: production
   ```

3. **配置多后端**:
   - 使用多个 LLM 后端实现高可用
   - 配置健康检查和自动故障转移

4. **优化 LLM 参数**:
   ```yaml
   parameters:
     temperature: 0.2
     max_tokens: 120
   ```

5. **监控配置**:
   - 定期检查 `/admin/metrics`
   - 配置日志收集
