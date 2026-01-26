// validation.go contains request validation for audio processing.
package audio

import (
	"fmt"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

// Validate 验证请求 - 实现 LogicHandler 接口
func (p *Processor) Validate(req ProcessRequest) error {
	if len(req.Audio) == 0 {
		return coreerrors.NewValidationError("audio data is required", nil)
	}

	if req.AudioFormat == "" {
		return coreerrors.NewValidationError("audio format is required", nil)
	}

	// 验证音频大小限制（32MB）
	maxSize := 32 * 1024 * 1024
	if len(req.Audio) > maxSize {
		return coreerrors.NewValidationError(
			fmt.Sprintf("audio size (%d bytes) exceeds maximum allowed size (%d bytes)", len(req.Audio), maxSize),
			nil,
		)
	}

	// 验证支持的格式
	supportedFormats := map[string]bool{
		"wav":  true,
		"mp3":  true,
		"m4a":  true,
		"opus": true,
		"flac": true,
	}

	if !supportedFormats[req.AudioFormat] {
		return coreerrors.NewValidationError(fmt.Sprintf("unsupported audio format: %s", req.AudioFormat), nil)
	}

	// 验证任务类型
	validTasks := map[prompt.TaskType]bool{
		prompt.TaskTranslate:  true,
		prompt.TaskTranscribe: true,
		// 保留用于后续扩展
		// prompt.TaskTranscribeAndTranslate: true,
	}

	if !validTasks[req.Task] {
		return coreerrors.NewValidationError(fmt.Sprintf("invalid task type: %s", req.Task), nil)
	}

	return nil
}
