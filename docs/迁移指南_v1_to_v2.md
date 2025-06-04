# Lingualink Core API 迁移指南 (v1.x → v2.0)

## 概述

Lingualink Core v2.0 引入了重大的架构改进和API变更。本指南将帮助你从 v1.x 版本迁移到 v2.0 版本。

## 主要变更总览

### 🔄 API端点变更
- `/api/v1/process` → `/api/v1/process_audio`
- 新增 `/api/v1/process_text`

### ✅ 保留的功能
- `transcribe` 任务类型（音频处理）
- `translate` 任务类型（音频处理）
- 所有原有的音频处理功能

### ✅ 新增功能
- 文本翻译API (`/process_text`)
- 改进的错误处理
- 更详细的响应格式
- 模块化的架构设计

## 详细迁移步骤

### 1. 更新API端点

#### 旧版本 (v1.x)
```bash
POST /api/v1/process
```

#### 新版本 (v2.0)
```bash
# 音频处理
POST /api/v1/process_audio

# 文本翻译 (新功能)
POST /api/v1/process_text
```

### 2. 任务类型保持不变

#### 音频处理支持的任务（无变化）
```json
{
  "task": "transcribe"  // ✅ 保留 - 仅转录
}
```

```json
{
  "task": "translate"   // ✅ 保留 - 转录+翻译
}
```

#### 文本处理支持的任务（新功能）
```json
{
  "task": "translate"   // 仅支持翻译
}
```

### 3. 更新请求参数

#### 音频处理请求（无变化）

**转录任务**:
```json
{
  "audio": "base64-data",
  "audio_format": "wav",
  "task": "transcribe"            // ✅ 保留支持
}
```

**翻译任务**:
```json
{
  "audio": "base64-data",
  "audio_format": "wav",
  "task": "translate",            // ✅ 保留支持
  "target_languages": ["en", "ja"] // 翻译任务需要
}
```

#### 新增文本翻译请求

**v2.0 新功能**:
```json
{
  "text": "需要翻译的文本",
  "target_languages": ["en", "ja"],
  "source_language": "zh"         // 可选
}
```

## 代码迁移示例

### JavaScript/Node.js

#### 旧版本代码
```javascript
// ❌ 旧版本 - 需要更新
async function processAudio(audioData, format, task, languages = []) {
  const response = await axios.post('/api/v1/process', {
    audio: audioData,
    audio_format: format,
    task: task,
    target_languages: languages
  });
  return response.data;
}

// 使用示例
const result1 = await processAudio(audioData, 'wav', 'transcribe');
const result2 = await processAudio(audioData, 'wav', 'translate', ['en']);
```

#### 新版本代码
```javascript
// ✅ 新版本 - 推荐写法
async function processAudio(audioData, format, languages) {
  const response = await axios.post('/api/v1/process_audio', {
    audio: audioData,
    audio_format: format,
    task: 'translate',
    target_languages: languages
  });
  return response.data;
}

async function processText(text, languages, sourceLanguage = null) {
  const payload = { text, target_languages: languages };
  if (sourceLanguage) payload.source_language = sourceLanguage;
  
  const response = await axios.post('/api/v1/process_text', payload);
  return response.data;
}

// 使用示例
const audioResult = await processAudio(audioData, 'wav', ['en', 'ja']);
const textResult = await processText('你好世界', ['en', 'ja']);
```

### Python

#### 旧版本代码
```python
# ❌ 旧版本 - 需要更新
def process_audio(audio_data, format, task, languages=None):
    payload = {
        'audio': audio_data,
        'audio_format': format,
        'task': task
    }
    if languages:
        payload['target_languages'] = languages
    
    response = requests.post('/api/v1/process', json=payload)
    return response.json()

# 使用示例
result1 = process_audio(audio_data, 'wav', 'transcribe')
result2 = process_audio(audio_data, 'wav', 'translate', ['en'])
```

#### 新版本代码
```python
# ✅ 新版本 - 推荐写法
def process_audio(audio_data, format, languages):
    payload = {
        'audio': audio_data,
        'audio_format': format,
        'task': 'translate',
        'target_languages': languages
    }
    response = requests.post('/api/v1/process_audio', json=payload)
    return response.json()

def process_text(text, languages, source_language=None):
    payload = {
        'text': text,
        'target_languages': languages
    }
    if source_language:
        payload['source_language'] = source_language
    
    response = requests.post('/api/v1/process_text', json=payload)
    return response.json()

# 使用示例
audio_result = process_audio(audio_data, 'wav', ['en', 'ja'])
text_result = process_text('你好世界', ['en', 'ja'])
```

## 响应格式变更

### 音频处理响应

响应格式基本保持不变，但有以下改进：

```json
{
  "request_id": "req_1704067200123456",
  "status": "success",
  "transcription": "转录文本",
  "translations": {
    "en": "English translation",
    "ja": "日本語翻訳"
  },
  "raw_response": "原文: ...\n英文: ...",
  "processing_time": 2.345,
  "metadata": {
    "model": "gpt-4",
    "prompt_tokens": 150,
    "total_tokens": 200,
    "backend": "openai"
  }
}
```

### 文本翻译响应 (新)

```json
{
  "request_id": "txt_1704067200123456",
  "status": "success",
  "source_text": "源文本",
  "translations": {
    "en": "English translation",
    "ja": "日本語翻訳"
  },
  "raw_response": "英文: ...\n日文: ...",
  "processing_time": 1.234,
  "metadata": {
    "model": "gpt-4",
    "prompt_tokens": 80,
    "total_tokens": 120,
    "backend": "openai"
  }
}
```

## 错误处理更新

### 新增错误类型

```json
{
  "error": "invalid task type: transcribe"
}
```

```json
{
  "error": "target languages are required"
}
```

```json
{
  "error": "text length (15000 characters) exceeds maximum allowed length (10000 characters)"
}
```

## 迁移检查清单

### ✅ 必须完成的更改

- [ ] 更新所有 `/process` 调用为 `/process_audio`
- [ ] 更新错误处理逻辑

### ✅ 推荐的改进

- [ ] 使用新的文本翻译API处理纯文本
- [ ] 更新客户端库到最新版本
- [ ] 添加新的错误类型处理
- [ ] 利用改进的响应格式

### ✅ 测试验证

- [ ] 测试音频处理功能
- [ ] 测试文本翻译功能
- [ ] 验证错误处理
- [ ] 性能基准测试

## 常见问题

### Q: 转录功能还支持吗？
A: 是的！v2.0 完全保留了转录功能。你可以继续使用：
```json
{
  "task": "transcribe"
}
```
这将只返回转录结果，不进行翻译。

### Q: 现有的API Key还能用吗？
A: 是的，API Key保持兼容，无需更改。

### Q: 响应时间有变化吗？
A: 文本翻译通常比音频处理更快。音频处理性能有所优化。

### Q: 如何处理大量文本？
A: 文本翻译有10,000字符限制。对于更长的文本，请分批处理。

## 获取帮助

如果在迁移过程中遇到问题：

1. 查看完整的 [API文档](./API_Documentation.md)
2. 运行测试脚本验证功能
3. 查看服务日志获取详细错误信息
4. 联系开发团队获取支持

---

**迁移完成后，你将获得：**
- 更清晰的API设计
- 保留所有原有音频处理功能（transcribe + translate）
- 文本翻译新功能
- 更好的性能和稳定性
- 模块化的架构设计
- 为未来多模态功能做好准备
