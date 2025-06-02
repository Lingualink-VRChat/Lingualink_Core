# 语言代码重构说明

## 概述

本次重构统一了API输入输出的语言代码格式，实现了以下目标：

1. **统一使用短代码**：API输入输出都使用标准的语言短代码（如 `zh`, `zh-hant`, `en`, `ja`）
2. **内部使用中文显示名称**：在构建LLM prompt时使用中文显示名称（如"中文"、"英文"、"日文"）
3. **移除客户端控制的用户提示词**：`user_prompt`字段被移除，改为服务端完全控制
4. **支持更多语言变体**：新增对繁体中文（`zh-hant`）等语言变体的支持
5. **简化模板配置**：移除无用的`template`相关配置，使用硬编码的默认模板

## 修改的文件

### 1. 配置文件 (`config/config.yaml`)

```yaml
prompt:
  defaults:
    task: translate
    target_languages: ["en", "ja", "zh"] # 改为使用短代码
  # 移除无用的template配置，使用硬编码的默认模板
  languages:
    - code: zh
      names:
        display: "中文"    # 用于LLM prompt的中文显示名称
        english: "Chinese"
        native: "中文"
    - code: zh-hant       # 新增繁体中文支持
      names:
        display: "繁體中文"
        english: "Traditional Chinese"
        native: "繁體中文"
    # ... 其他语言
```

### 2. 内部配置 (`internal/config/config.go`)

- 默认目标语言改为短代码：`[]string{"en", "ja", "zh"}`
- 默认任务类型改为 `"translate"`
- 移除无用的 `Template` 和 `TemplateDir` 字段

### 3. Prompt引擎 (`internal/core/prompt/engine.go`)

#### 主要变更：

- **输入处理**：`PromptRequest.TargetLanguages` 接收短代码
- **内部转换**：`convertCodesToDisplayNames()` 将短代码转换为中文显示名称用于构建LLM prompt
- **输出映射**：`ParseResponse()` 将LLM返回的中文名称键转换回短代码
- **移除用户提示词**：`PromptRequest` 结构体移除 `UserPrompt` 字段
- **简化模板处理**：移除 `Template` 字段，直接使用硬编码的"default"模板
- **移除模板注册**：删除 `RegisterTemplate()` 函数，简化架构
- **语言映射**：新增 `languageNameMap` 用于中文显示名称到短代码的反向映射

#### 数据流：

```
客户端输入: ["en", "ja", "zh"]
    ↓
内部转换: ["英文", "日文", "中文"] (用于LLM prompt)
    ↓
LLM输出: "英文: English text\n日文: Japanese text"
    ↓
解析转换: {"en": "English text", "ja": "Japanese text"}
    ↓
API输出: {"translations": {"en": "English text", "ja": "Japanese text"}}
```

### 4. 音频处理器 (`internal/core/audio/processor.go`)

- **移除用户提示词**：`ProcessRequest` 结构体移除 `UserPrompt` 字段
- **移除模板字段**：`ProcessRequest` 结构体移除 `Template` 字段
- **默认语言**：使用短代码作为默认目标语言 `[]string{"en", "ja", "zh"}`
- **响应处理**：正确处理短代码键的翻译结果

### 5. API处理器 (`internal/api/handlers/handlers.go`)

- **移除用户提示词处理**：不再接收和传递 `user_prompt` 参数
- **移除模板参数处理**：不再接收和传递 `template` 参数
- **语言列表API**：`ListSupportedLanguages()` 返回完整的语言信息（包含短代码、显示名称、别名）

### 6. 测试脚本

#### `test_audio.sh`
- 所有测试用例改为使用短代码（`zh`, `en`, `ja`, `zh-hant`）
- 移除 `user_prompt` 参数
- 移除 `template` 参数
- 新增繁体中文翻译测试

#### `test_api.sh`
- 音频处理测试改为使用短代码
- 移除 `user_prompt` 参数
- 移除 `template` 参数
- 任务类型统一为 `translate`

## API使用示例

### 表单方式

```bash
curl -X POST \
  -H "X-API-Key: dev-key-123" \
  -F "audio=@test.wav" \
  -F "task=translate" \
  -F "target_languages=en,ja,zh-hant" \
  "http://localhost:8080/api/v1/process"
```

### JSON方式

```bash
curl -X POST \
  -H "X-API-Key: dev-key-123" \
  -H "Content-Type: application/json" \
  -d '{
    "audio": "base64_encoded_audio",
    "audio_format": "wav",
    "task": "translate",
    "target_languages": ["en", "ja", "zh-hant"]
  }' \
  "http://localhost:8080/api/v1/process/json"
```

### 响应格式

```json
{
  "request_id": "req_123456789",
  "status": "success",
  "transcription": "原始音频转录内容",
  "translations": {
    "en": "English translation",
    "ja": "Japanese translation",
    "zh-hant": "繁體中文翻譯"
  },
  "raw_response": "LLM原始响应...",
  "processing_time": 2.5,
  "metadata": {...}
}
```

## 支持的语言

| 短代码 | 中文显示名称 | 英文名称 | 原生名称 |
|--------|-------------|---------|---------|
| `zh` | 中文 | Chinese | 中文 |
| `zh-hant` | 繁體中文 | Traditional Chinese | 繁體中文 |
| `en` | 英文 | English | English |
| `ja` | 日文 | Japanese | 日本語 |
| `ko` | 韩文 | Korean | 한국어 |
| `es` | 西班牙语 | Spanish | Español |
| `fr` | 法语 | French | Français |
| `de` | 德语 | German | Deutsch |

## 兼容性说明

### 破坏性变更

1. **API输入格式变更**：`target_languages` 必须使用短代码
2. **移除用户提示词**：不再接受 `user_prompt` 参数
3. **移除模板参数**：不再接受 `template` 参数，使用硬编码的默认模板
4. **API输出格式变更**：`translations` 对象的键改为短代码

### 迁移指南

对于现有客户端，需要进行以下修改：

1. 将目标语言从中文名称改为短代码：
   - `"英文"` → `"en"`
   - `"日文"` → `"ja"`
   - `"中文"` → `"zh"`

2. 移除 `user_prompt` 参数

3. 移除 `template` 参数

4. 更新响应处理逻辑以使用短代码键

## 测试

运行以下命令进行测试：

```bash
# 音频处理专项测试
./test_audio.sh

# 完整API测试
./test_api.sh
```

确保所有测试用例都使用新的短代码格式并且不包含 `user_prompt` 参数。 