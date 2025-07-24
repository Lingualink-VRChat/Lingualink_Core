# LLM 参数配置指南

本文档介绍如何在 Lingualink Core 中配置大语言模型的详细参数。

## 概述

Lingualink Core 现在支持在配置文件中详细配置 LLM 模型参数，包括温度、最大token数、重复惩罚等。这些参数可以在配置文件中设置默认值，也可以在API请求中动态覆盖。

## 配置结构

在 `config.yaml` 文件的 `backends.providers` 部分，每个后端提供者现在支持 `parameters` 字段：

```yaml
backends:
  providers:
    - name: default
      type: vllm
      url: http://localhost:8000/v1
      model: qwenOmni7
      api_key: "your-api-key"
      parameters:
        temperature: 0.7
        max_tokens: 1000
        # ... 其他参数
```

## 支持的参数

### 基础参数

- **temperature** (float, 0.0-2.0)
  - 控制输出的随机性
  - 0.0 = 完全确定性，2.0 = 最大随机性
  - 默认值：0.7

- **max_tokens** (int)
  - 最大输出token数
  - 控制响应的长度
  - 默认值：1000

- **top_p** (float, 0.0-1.0)
  - 核采样参数
  - 控制候选词的累积概率阈值
  - 默认值：无（由模型决定）

### 高级参数

- **top_k** (int)
  - Top-K采样参数
  - 限制候选词的数量
  - 某些模型支持

- **repetition_penalty** (float, 0.0-2.0)
  - 重复惩罚
  - 1.0 = 无惩罚，>1.0 = 惩罚重复
  - 主要用于开源模型

- **frequency_penalty** (float, -2.0-2.0)
  - 频率惩罚
  - 基于token在文本中出现的频率进行惩罚
  - OpenAI模型支持

- **presence_penalty** (float, -2.0-2.0)
  - 存在惩罚
  - 基于token是否已经出现进行惩罚
  - OpenAI模型支持

### 控制参数

- **stop** (array of strings)
  - 停止词列表
  - 遇到这些词时停止生成
  - 例如：`["END", "STOP", "<|endoftext|>"]`

- **seed** (int)
  - 随机种子
  - 用于生成可重现的结果
  - 某些模型支持

- **stream** (bool)
  - 是否启用流式输出
  - true = 流式，false = 一次性返回
  - 默认值：false

## 配置示例

### 创意写作配置
```yaml
parameters:
  temperature: 1.2      # 高创意性
  max_tokens: 2000      # 长输出
  top_p: 0.95          # 高多样性
  repetition_penalty: 1.1
```

### 精确翻译配置
```yaml
parameters:
  temperature: 0.1      # 低随机性
  max_tokens: 1500
  top_p: 0.8           # 中等多样性
  frequency_penalty: 0.2
```

### 对话聊天配置
```yaml
parameters:
  temperature: 0.7      # 平衡
  max_tokens: 1000
  top_p: 0.9
  seed: 42             # 可重现
  stream: true         # 流式输出
```

## API 请求中的参数覆盖

除了在配置文件中设置默认参数，还可以在API请求中动态覆盖这些参数：

```json
{
  "system_prompt": "你是一个翻译助手",
  "user_prompt": "请翻译这段文字",
  "options": {
    "temperature": 0.3,
    "max_tokens": 500,
    "top_p": 0.8
  }
}
```

请求中的参数会覆盖配置文件中的默认值。

## 不同模型的兼容性

| 参数 | OpenAI | VLLM | 其他OpenAI兼容 |
|------|--------|------|----------------|
| temperature | ✅ | ✅ | ✅ |
| max_tokens | ✅ | ✅ | ✅ |
| top_p | ✅ | ✅ | ✅ |
| top_k | ❌ | ✅ | 取决于实现 |
| repetition_penalty | ❌ | ✅ | 取决于实现 |
| frequency_penalty | ✅ | ❌ | 取决于实现 |
| presence_penalty | ✅ | ❌ | 取决于实现 |
| stop | ✅ | ✅ | ✅ |
| seed | ✅ | ✅ | 取决于实现 |
| stream | ✅ | ✅ | ✅ |

## 最佳实践

1. **根据用途调整参数**：
   - 创意任务：高 temperature (0.8-1.2)
   - 精确任务：低 temperature (0.1-0.3)
   - 平衡任务：中等 temperature (0.5-0.8)

2. **合理设置 max_tokens**：
   - 根据预期输出长度设置
   - 避免设置过大导致不必要的计算

3. **使用 seed 确保可重现性**：
   - 在测试和调试时使用固定种子
   - 生产环境可以不设置以获得多样性

4. **根据模型选择合适的参数**：
   - OpenAI模型：使用 frequency_penalty 和 presence_penalty
   - 开源模型：使用 repetition_penalty 和 top_k

## 故障排除

如果参数不生效，请检查：

1. 参数名称是否正确
2. 参数值是否在有效范围内
3. 使用的模型是否支持该参数
4. 配置文件语法是否正确

查看日志可以帮助诊断参数传递问题。
