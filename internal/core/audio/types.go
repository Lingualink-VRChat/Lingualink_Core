// types.go defines request/response payloads for audio processing.
package audio

import (
	"fmt"
	"sync"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
)

// ProcessRequest 音频处理请求
type ProcessRequest struct {
	Audio           []byte                  `json:"audio"`
	AudioFormat     string                  `json:"audio_format"`
	Task            prompt.TaskType         `json:"task"`
	SourceLanguage  string                  `json:"source_language,omitempty"`
	TargetLanguages []string                `json:"target_languages"` // 接收短代码
	UserDictionary  []config.DictionaryTerm `json:"user_dictionary,omitempty"`
	// 移除Template字段，使用硬编码的默认模板
	// 移除 UserPrompt，改为服务端控制
	Options map[string]interface{} `json:"options,omitempty"`

	cleanup     func()
	cleanupOnce *sync.Once
}

// GetTargetLanguages 实现 ProcessableRequest 接口
func (req ProcessRequest) GetTargetLanguages() []string {
	return req.TargetLanguages
}

// SetCleanup registers a callback that will be executed after the LLM request is finished.
// It can be used to release large temporary buffers.
func (req *ProcessRequest) SetCleanup(fn func()) {
	req.cleanup = fn
	if fn != nil {
		req.cleanupOnce = new(sync.Once)
	} else {
		req.cleanupOnce = nil
	}
}

// Cleanup executes the registered cleanup callback at most once.
// It is safe to call multiple times across copies of ProcessRequest.
func (req ProcessRequest) Cleanup() {
	if req.cleanupOnce == nil || req.cleanup == nil {
		return
	}
	req.cleanupOnce.Do(req.cleanup)
}

// ProcessResponse 音频处理响应
type ProcessResponse struct {
	RequestID      string                 `json:"request_id"`
	Status         string                 `json:"status"`
	Transcription  string                 `json:"transcription,omitempty"`
	CorrectedText  string                 `json:"corrected_text,omitempty"`
	Translations   map[string]string      `json:"translations,omitempty"` // 键为短代码
	RawResponse    string                 `json:"raw_response"`
	ProcessingTime float64                `json:"processing_time"`
	Metadata       map[string]interface{} `json:"metadata"`
}

func (r *ProcessResponse) SetProcessingTime(seconds float64) {
	r.ProcessingTime = seconds
}

func (r *ProcessResponse) SetRequestID(requestID string) {
	r.RequestID = requestID
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}
