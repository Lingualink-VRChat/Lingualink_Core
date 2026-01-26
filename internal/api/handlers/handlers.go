package handlers

import (
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/audio"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/processing"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/text"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/auth"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

// Handler API处理器
type Handler struct {
	config                 *config.Config
	llmManager             *llm.Manager
	asrManager             *asr.Manager
	startTime              time.Time
	version                string
	audioProcessor         *audio.Processor
	textProcessor          *text.Processor
	audioProcessingService *processing.Service[audio.ProcessRequest, *audio.ProcessResponse]
	textProcessingService  *processing.Service[text.ProcessRequest, *text.ProcessResponse]
	statusStore            processing.StatusStore
	authenticator          *auth.MultiAuthenticator
	logger                 *logrus.Logger
	metrics                metrics.MetricsCollector
}

// NewHandler 创建API处理器
func NewHandler(
	audioProcessor *audio.Processor,
	textProcessor *text.Processor,
	audioProcessingService *processing.Service[audio.ProcessRequest, *audio.ProcessResponse],
	textProcessingService *processing.Service[text.ProcessRequest, *text.ProcessResponse],
	statusStore processing.StatusStore,
	authenticator *auth.MultiAuthenticator,
	logger *logrus.Logger,
	metrics metrics.MetricsCollector,
	cfg *config.Config,
	llmManager *llm.Manager,
	asrManager *asr.Manager,
) *Handler {
	return &Handler{
		config:                 cfg,
		llmManager:             llmManager,
		asrManager:             asrManager,
		startTime:              time.Now(),
		version:                "1.0.0",
		audioProcessor:         audioProcessor,
		textProcessor:          textProcessor,
		audioProcessingService: audioProcessingService,
		textProcessingService:  textProcessingService,
		statusStore:            statusStore,
		authenticator:          authenticator,
		logger:                 logger,
		metrics:                metrics,
	}
}
