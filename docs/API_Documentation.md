# Lingualink Core API 文档

## 概述

Lingualink Core 是一个高性能的音频处理服务，提供音频转录和翻译功能。本文档详细描述了所有可用的 API 端点、请求格式、响应格式和错误处理。

## 基础信息

- **基础URL**: `http://localhost:8080/api/v1`
- **API版本**: v1.1
- **内容类型**: `application/json`
- **认证方式**: API Key 或 JWT Token

## 认证

### API Key 认证

在请求头中包含 API Key：

```http
X-API-Key: your-api-key-here
```

或者在 Authorization 头中：

```http
Authorization: ApiKey your-api-key-here
```

### JWT Token 认证

```http
Authorization: Bearer your-jwt-token-here
```

### 身份类型

- **user**: 普通用户，有基本的音频处理权限
- **service**: 服务级别，有管理员权限
- **anonymous**: 匿名用户，仅能访问健康检查

## API 端点

### 1. 健康检查

检查服务状态和健康信息。

**端点**: `GET /health`  
**认证**: 不需要  
**权限**: 公开访问

#### 请求示例

```bash
curl -X GET "http://localhost:8080/api/v1/health"
```

#### 响应示例

```json
{
  "status": "healthy",
  "timestamp": 1704067200,
  "version": "1.0.0"
}
```

#### 详细健康检查

添加 `detailed=true` 参数获取详细信息：

```bash
curl -X GET "http://localhost:8080/api/v1/health?detailed=true"
```

```json
{
  "status": "healthy",
  "timestamp": 1704067200,
  "version": "1.0.0",
  "services": {
    "audio_processor": "healthy",
    "llm_manager": "healthy",
    "prompt_engine": "healthy"
  }
}
```

### 2. 系统能力查询

获取系统支持的功能和限制。

**端点**: `GET /capabilities`  
**认证**: 需要  
**权限**: 任何已认证用户

#### 请求示例

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/capabilities"
```

#### 响应示例

```json
{
  "supported_formats": ["wav", "mp3", "m4a", "opus", "flac"],
  "max_audio_size": 33554432,
  "supported_tasks": ["transcribe", "translate"],
  "supported_languages": ["zh", "zh-hant", "en", "ja", "ko", "es", "fr", "de", "ru", "it"],
  "audio_conversion": true,
  "conversion_metrics": {
    "total_conversions": 0,
    "successful_conversions": 0,
    "failed_conversions": 0,
    "average_conversion_time": 0
  }
}
```

### 3. 支持的语言列表

获取所有支持的语言及其详细信息。

**端点**: `GET /languages`  
**认证**: 需要  
**权限**: 任何已认证用户

#### 请求示例

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/languages"
```

#### 响应示例

```json
{
  "languages": [
    {
      "code": "zh",
      "display": "中文",
      "aliases": ["chinese", "中文", "汉语", "zh-cn"]
    },
    {
      "code": "zh-hant",
      "display": "繁體中文",
      "aliases": ["zh-tw", "zh-hk", "traditional chinese", "繁体中文", "繁體中文"]
    },
    {
      "code": "en",
      "display": "英文",
      "aliases": ["english", "英文", "英语"]
    },
    {
      "code": "ja",
      "display": "日文",
      "aliases": ["japanese", "日文", "日语", "日本語"]
    },
    {
      "code": "ko",
      "display": "韩文",
      "aliases": ["korean", "韩文", "韩语", "한국어"]
    },
    {
      "code": "es",
      "display": "西班牙语",
      "aliases": ["spanish", "西班牙语", "español"]
    },
    {
      "code": "fr",
      "display": "法语",
      "aliases": ["french", "法语", "français"]
    },
    {
      "code": "de",
      "display": "德语",
      "aliases": ["german", "德语", "deutsch"]
    },
    {
      "code": "ru",
      "display": "俄语",
      "aliases": ["russian", "俄语", "俄文", "俄罗斯语"]
    },
    {
      "code": "it",
      "display": "意大利语",
      "aliases": ["italian", "意大利语", "意大利文"]
    }
  ],
  "count": 10
}
```

### 4. 音频处理

使用 JSON 格式处理 base64 编码的音频数据。

**端点**: `POST /process`
**认证**: 需要
**权限**: `audio.process`, `audio.transcribe`, `audio.translate`
**内容类型**: `application/json`

#### 任务类型说明

- **`transcribe`**: 仅转录任务，将音频内容转录成其原始语言的文本，不进行翻译
- **`translate`**: 转录+翻译任务，首先转录音频内容，然后翻译成指定的目标语言

#### 请求体

```json
{
  "audio": "base64-encoded-audio-data",
  "audio_format": "wav",
  "task": "transcribe",
  "source_language": "zh",
  "target_languages": ["en", "ja"],
  "options": {}
}
```

#### 请求参数说明

| 字段 | 类型 | 必需 | 描述 |
|------|------|------|------|
| `audio` | string | 是 | Base64编码的音频数据 |
| `audio_format` | string | 是 | 音频格式 (wav, mp3, m4a, opus, flac) |
| `task` | string | 是 | 任务类型: `transcribe` (仅转录) 或 `translate` (转录+翻译) |
| `source_language` | string | 否 | 源语言代码，通常由系统自动检测 |
| `target_languages` | array | 否 | 目标语言代码数组，仅在 `translate` 任务时需要 |
| `options` | object | 否 | 额外选项，预留字段 |

#### 请求示例

**转录任务** (仅转录，不翻译):
```bash
curl -X POST \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "audio": "UklGRiQAAABXQVZFZm10IBAAAAABAAEA...",
    "audio_format": "wav",
    "task": "transcribe"
  }' \
  "http://localhost:8080/api/v1/process"
```

**翻译任务** (转录+翻译):
```bash
curl -X POST \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "audio": "UklGRiQAAABXQVZFZm10IBAAAAABAAEA...",
    "audio_format": "wav",
    "task": "translate",
    "target_languages": ["en", "ja"]
  }' \
  "http://localhost:8080/api/v1/process"
```

### 音频处理响应格式

音频处理端点返回以下格式：

```json
{
  "request_id": "req_1704067200123456",
  "status": "success",
  "transcription": "这是转录的文本内容",
  "translations": {
    "en": "This is the transcribed text content",
    "ja": "これは転写されたテキストの内容です"
  },
  "raw_response": "原文: 这是转录的文本内容\n英文: This is...",
  "processing_time": 2.345,
  "metadata": {
    "model": "gpt-4",
    "prompt_tokens": 150,
    "total_tokens": 200,
    "backend": "openai",
    "original_format": "wav",
    "processed_format": "wav",
    "conversion_applied": false
  }
}
```

#### 响应字段说明

| 字段 | 类型 | 描述 |
|------|------|------|
| `request_id` | string | 请求唯一标识符 |
| `status` | string | 处理状态: `success`, `partial_success`, `failed` |
| `transcription` | string | 转录文本，所有任务都会返回 |
| `translations` | object | 翻译结果，键为语言代码，仅 `translate` 任务返回 |
| `raw_response` | string | LLM原始响应 |
| `processing_time` | number | 处理时间(秒) |
| `metadata` | object | 处理元数据 |

#### 不同任务类型的响应差异

**转录任务响应** (`task: "transcribe"`):
```json
{
  "request_id": "req_1704067200123456",
  "status": "success",
  "transcription": "这是转录的文本内容",
  "translations": {},
  "raw_response": "原文: 这是转录的文本内容",
  "processing_time": 1.234,
  "metadata": { ... }
}
```

**翻译任务响应** (`task: "translate"`):
```json
{
  "request_id": "req_1704067200123456",
  "status": "success",
  "transcription": "这是转录的文本内容",
  "translations": {
    "en": "This is the transcribed text content",
    "ja": "これは転写されたテキストの内容です"
  },
  "raw_response": "原文: 这是转录的文本内容\n英文: This is...",
  "processing_time": 2.345,
  "metadata": { ... }
}
```

### 5. 处理状态查询

查询异步处理任务的状态（预留功能）。

**端点**: `GET /status/{request_id}`  
**认证**: 需要  
**权限**: 任何已认证用户

#### 请求示例

```bash
curl -X GET \
  -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/status/req_1704067200123456"
```

#### 响应示例

```json
{
  "request_id": "req_1704067200123456",
  "status": "completed",
  "progress": 100,
  "message": "Processing completed"
}
```

### 6. 监控指标

获取系统监控指标（管理员功能）。

**端点**: `GET /admin/metrics`  
**认证**: 需要  
**权限**: 服务级别认证

#### 请求示例

```bash
curl -X GET \
  -H "X-API-Key: service-api-key" \
  "http://localhost:8080/api/v1/admin/metrics"
```

#### 响应示例

```json
{
  "counters": {
    "api.process_audio.success": 150,
    "api.process_audio_json.success": 75,
    "audio.process.success": 225,
    "http_requests_total": 500
  },
  "histograms": {
    "audio.process": {
      "count": 225,
      "sum": 450.5,
      "avg": 2.002
    },
    "http_request_duration": {
      "count": 500,
      "sum": 125.3,
      "avg": 0.251
    }
  }
}
```

## 错误处理

### HTTP 状态码

- `200 OK`: 请求成功
- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: 认证失败
- `403 Forbidden`: 权限不足
- `404 Not Found`: 资源不存在
- `413 Payload Too Large`: 文件过大
- `422 Unprocessable Entity`: 请求格式正确但无法处理
- `429 Too Many Requests`: 请求频率超限
- `500 Internal Server Error`: 服务器内部错误

### 错误响应格式

```json
{
  "error": "错误描述信息"
}
```

### 常见错误

#### 认证错误

```json
{
  "error": "authentication failed"
}
```

#### 权限错误

```json
{
  "error": "insufficient permissions"
}
```

#### 参数错误

```json
{
  "error": "audio data is required"
}
```

#### 文件大小错误

```json
{
  "error": "audio size (35000000 bytes) exceeds maximum allowed size (33554432 bytes)"
}
```

#### 格式错误

```json
{
  "error": "unsupported audio format: xyz"
}
```

#### 任务类型错误

```json
{
  "error": "invalid task type: unknown"
}
```

## 限制和约束

### 文件大小限制

- 最大音频文件大小: **32MB**

### 支持的音频格式

- **WAV**: 推荐格式，无需转换
- **MP3**: 支持，会自动转换为WAV
- **M4A**: 支持，会自动转换为WAV  
- **OPUS**: 支持，会自动转换为WAV
- **FLAC**: 支持，会自动转换为WAV

### 任务类型

- **transcribe**: 仅转录，将音频内容转录成其原始语言的文本，不进行翻译
- **translate**: 转录+翻译，首先转录音频内容，然后翻译成指定的目标语言

### 语言代码

使用标准的语言代码：
- `zh`: 简体中文
- `zh-hant`: 繁体中文
- `en`: 英文
- `ja`: 日文
- `ko`: 韩文
- `es`: 西班牙语
- `fr`: 法语
- `de`: 德语
- `ru`: 俄语
- `it`: 意大利语

### 频率限制

根据API Key配置，不同用户有不同的频率限制：
- 普通用户: 通常有每分钟请求数限制
- 服务级别: 可能无限制或更高限制
- 匿名用户: 每分钟10次请求

## SDK 和集成示例

### JavaScript/Node.js

```javascript
const fs = require('fs');
const axios = require('axios');

// 音频处理函数
async function processAudio(audioFilePath, audioFormat, task, targetLanguages = []) {
  // 读取音频文件并转换为base64
  const audioBuffer = fs.readFileSync(audioFilePath);
  const audioBase64 = audioBuffer.toString('base64');

  const response = await axios.post(
    'http://localhost:8080/api/v1/process',
    {
      audio: audioBase64,
      audio_format: audioFormat,
      task: task,
      target_languages: targetLanguages
    },
    {
      headers: {
        'X-API-Key': 'your-api-key',
        'Content-Type': 'application/json'
      }
    }
  );

  return response.data;
}

// 使用示例
async function example() {
  try {
    // 转录任务 - 仅转录，不翻译
    const transcribeResult = await processAudio('audio.wav', 'wav', 'transcribe');
    console.log('转录结果:', transcribeResult.transcription);
    console.log('翻译结果:', transcribeResult.translations); // 空对象 {}

    // 翻译任务 - 转录+翻译
    const translateResult = await processAudio('audio.wav', 'wav', 'translate', ['en', 'ja']);
    console.log('转录结果:', translateResult.transcription);
    console.log('翻译结果:', translateResult.translations);
  } catch (error) {
    console.error('处理失败:', error.response?.data || error.message);
  }
}
```

### Python

```python
import requests
import base64

def process_audio(audio_file_path, audio_format, task, target_languages=None):
    """
    处理音频文件

    Args:
        audio_file_path: 音频文件路径
        audio_format: 音频格式 (wav, mp3, m4a, opus, flac)
        task: 任务类型 (transcribe, translate)
        target_languages: 目标语言列表 (仅translate任务需要)

    Returns:
        dict: API响应结果
    """
    # 读取音频文件并编码为base64
    with open(audio_file_path, 'rb') as f:
        audio_data = base64.b64encode(f.read()).decode('utf-8')

    payload = {
        'audio': audio_data,
        'audio_format': audio_format,
        'task': task
    }

    if target_languages:
        payload['target_languages'] = target_languages

    response = requests.post(
        'http://localhost:8080/api/v1/process',
        json=payload,
        headers={
            'X-API-Key': 'your-api-key',
            'Content-Type': 'application/json'
        }
    )

    response.raise_for_status()  # 抛出HTTP错误
    return response.json()

# 使用示例
if __name__ == '__main__':
    try:
        # 转录任务 - 仅转录，不翻译
        transcribe_result = process_audio('audio.wav', 'wav', 'transcribe')
        print(f"转录结果: {transcribe_result['transcription']}")
        print(f"翻译结果: {transcribe_result['translations']}")  # 空字典 {}

        # 翻译任务 - 转录+翻译
        translate_result = process_audio('audio.wav', 'wav', 'translate', ['en', 'ja'])
        print(f"转录结果: {translate_result['transcription']}")
        print(f"翻译结果: {translate_result['translations']}")

    except requests.exceptions.RequestException as e:
        print(f"请求失败: {e}")
    except KeyError as e:
        print(f"响应格式错误: {e}")
```

### cURL 脚本示例

```bash
#!/bin/bash

API_KEY="your-api-key"
BASE_URL="http://localhost:8080/api/v1"

# 函数：将音频文件转换为base64
audio_to_base64() {
    local file=$1
    if [[ -f "$file" ]]; then
        base64 -i "$file" | tr -d '\n'
    else
        echo "文件不存在: $file" >&2
        return 1
    fi
}

# 转录任务 - 仅转录，不翻译
echo "=== 转录测试 ==="
AUDIO_BASE64=$(audio_to_base64 "test.wav")
if [[ $? -eq 0 ]]; then
    curl -X POST \
      -H "X-API-Key: $API_KEY" \
      -H "Content-Type: application/json" \
      -d "{
        \"audio\": \"$AUDIO_BASE64\",
        \"audio_format\": \"wav\",
        \"task\": \"transcribe\"
      }" \
      "$BASE_URL/process" | jq .
fi

# 翻译任务 - 转录+翻译
echo "=== 翻译测试 ==="
if [[ $? -eq 0 ]]; then
    curl -X POST \
      -H "X-API-Key: $API_KEY" \
      -H "Content-Type: application/json" \
      -d "{
        \"audio\": \"$AUDIO_BASE64\",
        \"audio_format\": \"wav\",
        \"task\": \"translate\",
        \"target_languages\": [\"en\", \"ja\"]
      }" \
      "$BASE_URL/process" | jq .
fi

# 获取系统能力
echo "=== 系统能力 ==="
curl -X GET \
  -H "X-API-Key: $API_KEY" \
  "$BASE_URL/capabilities" | jq .

# 获取支持语言
echo "=== 支持语言 ==="
curl -X GET \
  -H "X-API-Key: $API_KEY" \
  "$BASE_URL/languages" | jq .
```

## 最佳实践

### 1. 音频文件优化

- **推荐格式**: WAV格式可避免FFmpeg转换开销，处理更快
- **文件大小**: 控制在32MB以内
- **音频质量**: 建议16kHz采样率，16位深度，单声道
- **格式转换**: 非WAV格式会自动转换，但会增加处理时间
- **Base64编码**: 注意base64编码会增加约33%的数据大小

### 2. 错误处理

- 始终检查HTTP状态码
- 解析错误响应中的具体错误信息
- 实现重试机制处理临时错误

### 3. 性能优化

- 对于大量请求，考虑使用连接池
- 实现客户端缓存避免重复请求
- 监控处理时间，优化音频文件

### 4. 安全考虑

- 妥善保管API Key，不要在客户端代码中硬编码
- 使用HTTPS传输敏感音频数据
- 定期轮换API Key

### 5. 监控和调试

- 记录request_id用于问题追踪
- 监控API响应时间和错误率
- 使用详细健康检查监控服务状态

## 更新日志

### v1.1.0 (当前版本)

- **任务类型优化**: 明确区分 `transcribe` (仅转录) 和 `translate` (转录+翻译) 任务
- **语言支持扩展**: 新增支持韩文、西班牙语、法语、德语、俄语、意大利语
- **API端点统一**: 统一使用 `/process` 端点，支持JSON格式请求
- **响应格式优化**:
  - `transcribe` 任务返回空的 `translations` 对象
  - `translate` 任务返回完整的转录和翻译结果
- **语言代码标准化**: 使用标准语言代码 (如 `zh`, `en`, `ja` 等)
- **提示词引擎优化**: 服务端控制提示词生成，提高一致性

### v1.0.0

- 初始API版本
- 支持音频转录和翻译
- 多种认证方式
- 完整的错误处理
- 监控和指标收集

---

如需更多帮助或有问题，请联系开发团队或查看项目文档。
