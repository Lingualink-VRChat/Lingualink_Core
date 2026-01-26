// llm_success_response.go contains the legacy LLM response -> API response adapter.
package audio

import (
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
)

// BuildSuccessResponse 构建成功响应 - 实现 LogicHandler 接口
func (p *Processor) BuildSuccessResponse(llmResp *llm.LLMResponse, parsedResp *prompt.ParsedResponse, req ProcessRequest) *ProcessResponse {
	requestID := generateRequestID()

	response := acquireProcessResponse()
	response.RequestID = requestID
	response.Status = "success"
	response.RawResponse = llmResp.Content
	response.ProcessingTime = 0 // 这将在 Service 中设置
	response.Metadata["model"] = llmResp.Model
	response.Metadata["prompt_tokens"] = llmResp.PromptTokens
	response.Metadata["total_tokens"] = llmResp.TotalTokens
	response.Metadata["backend"] = llmResp.Metadata["backend"]
	response.Metadata["original_format"] = req.AudioFormat
	response.Transcription = ""
	response.CorrectedText = ""

	// 添加解析器信息到元数据
	if parsedResp != nil && parsedResp.Metadata != nil {
		if parser, ok := parsedResp.Metadata["parser"]; ok {
			response.Metadata["parser"] = parser
		}
		if parseSuccess, ok := parsedResp.Metadata["parse_success"]; ok {
			response.Metadata["parse_success"] = parseSuccess
		}
	}

	// 如果解析失败，标记为部分成功
	if parsedResp == nil || parsedResp.Metadata["parse_error"] != nil {
		response.Status = "partial_success"
	}

	// 提取 ASR 转录
	if llmResp != nil && llmResp.Metadata != nil {
		if ctxAny, ok := llmResp.Metadata["context"]; ok {
			if ctxMap, ok := ctxAny.(map[string]interface{}); ok {
				if v, ok := ctxMap["asr_text"].(string); ok {
					response.Transcription = v
				}
				if v, ok := ctxMap["asr_language"].(string); ok && v != "" {
					response.Metadata["asr_language"] = v
				}
				if v, ok := ctxMap["asr_duration_ms"]; ok {
					response.Metadata["asr_duration_ms"] = v
				}
				if v, ok := ctxMap["audio_processed_format"]; ok {
					response.Metadata["processed_format"] = v
				}
				if v, ok := ctxMap["conversion_applied"]; ok {
					response.Metadata["conversion_applied"] = v
				}
			}
		}
	}

	if parsedResp != nil && parsedResp.CorrectedText != "" {
		response.CorrectedText = parsedResp.CorrectedText
	}

	sourceLangForMetrics := req.SourceLanguage
	if sourceLangForMetrics == "" {
		if v, ok := response.Metadata["asr_language"].(string); ok {
			sourceLangForMetrics = v
		}
	}
	if req.Task == prompt.TaskTranscribe && response.Transcription != "" {
		metrics.IncTranscription(sourceLangForMetrics)
	}

	// 提取翻译结果
	targetLangCodes := req.TargetLanguages
	if parsedResp != nil {
		for langCode, translationText := range parsedResp.Sections {
			// 验证这是一个我们期望的目标语言代码
			isTargetLang := false
			for _, targetCode := range targetLangCodes {
				if langCode == targetCode {
					isTargetLang = true
					break
				}
			}
			if isTargetLang {
				response.Translations[langCode] = translationText
				if req.Task == prompt.TaskTranslate {
					metrics.IncTranslation(sourceLangForMetrics, langCode)
				}
			} else {
				p.logger.Warnf("Unexpected section key '%s' found after parsing, not adding to translations.", langCode)
			}
		}
	}

	return response
}
