// audio.go contains audio processing endpoints.
package handlers

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/gin-gonic/gin"
)

// ProcessAudioJSON 处理JSON格式的音频请求
func (h *Handler) ProcessAudioJSON(c *gin.Context) {
	h.logger.Info("Processing JSON audio request")

	decoder := func(c *gin.Context) (audio.ProcessRequest, error) {
		var req struct {
			Audio           string                  `json:"audio"` // base64编码的音频数据
			AudioFormat     string                  `json:"audio_format"`
			Task            prompt.TaskType         `json:"task"`
			SourceLanguage  string                  `json:"source_language,omitempty"`
			TargetLanguages []string                `json:"target_languages"` // 期望短代码
			UserDictionary  []config.DictionaryTerm `json:"user_dictionary,omitempty"`
			Options         map[string]interface{}  `json:"options,omitempty"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			return audio.ProcessRequest{}, fmt.Errorf("invalid JSON: %w", err)
		}
		decodedLen := base64.StdEncoding.DecodedLen(len(req.Audio))
		if decodedLen > 32*1024*1024 {
			return audio.ProcessRequest{}, fmt.Errorf("audio size exceeds maximum allowed size")
		}
		buf := audio.AcquireAudioBuffer(decodedLen)
		base64Decoder := base64.NewDecoder(base64.StdEncoding, strings.NewReader(req.Audio))
		n := 0
		for n < len(buf) {
			readN, readErr := base64Decoder.Read(buf[n:])
			n += readN
			if readErr == nil {
				continue
			}
			if errors.Is(readErr, io.EOF) {
				break
			}
			audio.ReleaseAudioBuffer(buf)
			return audio.ProcessRequest{}, fmt.Errorf("invalid base64 audio data: %w", readErr)
		}
		audioReq := audio.ProcessRequest{
			Audio:           buf[:n],
			AudioFormat:     req.AudioFormat,
			Task:            req.Task,
			SourceLanguage:  req.SourceLanguage,
			TargetLanguages: req.TargetLanguages,
			UserDictionary:  req.UserDictionary,
			Options:         req.Options,
		}
		audioReq.SetCleanup(func() { audio.ReleaseAudioBuffer(buf) })
		return audioReq, nil
	}

	handleProcessingRequest(c, h, h.audioProcessingService, h.audioProcessor, decoder)
}
