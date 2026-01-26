// llm_request.go contains the legacy single-call LLM request builder.
package audio

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/correction"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/sirupsen/logrus"
)

// BuildLLMRequest 构建LLM请求 - 实现 LogicHandler 接口
func (p *Processor) BuildLLMRequest(ctx context.Context, req ProcessRequest) (*llm.LLMRequest, error) {
	requestID, _ := logging.RequestIDFromContext(ctx)

	// 1. 验证音频数据
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		entry := p.logger.WithError(err)
		if requestID != "" {
			entry = entry.WithField(logging.FieldRequestID, requestID)
		}
		entry.Warn("Audio validation failed, proceeding anyway")
	}

	// 2. 转换音频格式（如果需要）
	audioData := req.Audio
	audioFormat := req.AudioFormat

	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Warn("Audio conversion failed, using original format")
		} else {
			req.Cleanup()
			audioData = convertedData
			audioFormat = "wav"
			fields := logrus.Fields{
				"original_format":  req.AudioFormat,
				"converted_format": "wav",
			}
			if requestID != "" {
				fields[logging.FieldRequestID] = requestID
			}
			p.logger.WithFields(fields).Info("Audio converted successfully")
		}
	}

	// 3. 处理目标语言（使用短代码）
	targetLangCodes := req.TargetLanguages
	// 只有在translate任务且没有指定目标语言时，才使用默认目标语言
	if req.Task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages // 从配置获取默认目标语言
	}

	// 4. Stage 1: ASR 转录
	if p.asrManager == nil {
		return nil, coreerrors.NewInternalError("asr manager not configured", nil)
	}

	asrStart := time.Now()
	asrResp, err := p.asrManager.Transcribe(ctx, &asr.ASRRequest{
		Audio:       audioData,
		AudioFormat: audioFormat,
		Language:    req.SourceLanguage,
	})
	if err != nil {
		return nil, err
	}

	if req.Task == prompt.TaskTranscribe && !p.correction.Enabled {
		return nil, coreerrors.NewInternalError("transcribe without correction should be handled by direct processing", nil)
	}
	if req.Task == prompt.TaskTranslate && p.correction.Enabled && !p.correction.MergeWithTranslation {
		return nil, coreerrors.NewInternalError("separated correction+translation should be handled by direct processing", nil)
	}

	dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

	// 5. Stage 2/3: 构建 LLM 提示词
	var promptObj *prompt.Prompt
	switch req.Task {
	case prompt.TaskTranscribe:
		promptObj, err = p.promptEngine.BuildTextCorrectPrompt(ctx, asrResp.Text, dictionary)
	case prompt.TaskTranslate:
		if p.correction.Enabled && p.correction.MergeWithTranslation {
			promptObj, err = p.promptEngine.BuildTextCorrectTranslatePrompt(ctx, asrResp.Text, targetLangCodes, dictionary)
		} else {
			promptObj, err = p.promptEngine.BuildTextPrompt(ctx, prompt.PromptRequest{
				Task:            prompt.TaskTranslate,
				SourceLanguage:  req.SourceLanguage,
				TargetLanguages: targetLangCodes,
				Variables: map[string]interface{}{
					"source_text": asrResp.Text,
				},
			})
		}
	default:
		return nil, coreerrors.NewValidationError(fmt.Sprintf("unsupported task type: %s", req.Task), nil)
	}
	if err != nil {
		var appErr *coreerrors.AppError
		if errors.As(err, &appErr) {
			return nil, appErr
		}
		return nil, coreerrors.NewInternalError("build prompt failed", err)
	}

	llmReq := &llm.LLMRequest{
		SystemPrompt: promptObj.System,
		UserPrompt:   promptObj.User,
		Options:      req.Options,
		Context: map[string]interface{}{
			"asr_text":               asrResp.Text,
			"asr_language":           asrResp.DetectedLanguage,
			"asr_duration_ms":        time.Since(asrStart).Milliseconds(),
			"audio_original_format":  req.AudioFormat,
			"audio_processed_format": audioFormat,
			"conversion_applied":     audioFormat != req.AudioFormat,
		},
	}

	return llmReq, nil
}
