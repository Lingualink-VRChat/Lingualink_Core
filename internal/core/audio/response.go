// response.go contains pipeline execution and response construction for audio processing.
package audio

import (
	"context"
	"fmt"
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/correction"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/pipeline"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/prompt"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/tool"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/logging"
	"github.com/Lingualink-VRChat/Lingualink_Core/pkg/metrics"
	"github.com/sirupsen/logrus"
)

func (p *Processor) processWithPipeline(ctx context.Context, req ProcessRequest) (*ProcessResponse, error) {
	requestID, _ := logging.RequestIDFromContext(ctx)

	if err := p.ensurePipelineInitialized(); err != nil {
		return nil, err
	}

	// Validate audio data (best-effort).
	if err := p.audioConverter.ValidateAudioData(req.Audio, req.AudioFormat); err != nil {
		entry := p.logger.WithError(err)
		if requestID != "" {
			entry = entry.WithField(logging.FieldRequestID, requestID)
		}
		entry.Warn("Audio validation failed, proceeding anyway")
	}

	// Convert audio if needed.
	audioData := req.Audio
	audioFormat := req.AudioFormat
	conversionApplied := false

	if p.audioConverter.IsConversionNeeded(req.AudioFormat) {
		convertedData, err := p.audioConverter.ConvertToWAV(req.Audio, req.AudioFormat)
		if err != nil {
			p.logger.WithError(err).Warn("Audio conversion failed, using original format")
		} else {
			req.Cleanup()
			audioData = convertedData
			audioFormat = "wav"
			conversionApplied = true
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

	targetLangCodes := req.TargetLanguages
	if req.Task == prompt.TaskTranslate && len(targetLangCodes) == 0 {
		targetLangCodes = p.config.Defaults.TargetLanguages
	}

	dictionary := correction.MergeDictionaries(p.correction.GlobalDictionary, req.UserDictionary)

	selected, err := p.selectPipeline(req.Task)
	if err != nil {
		return nil, err
	}

	pctx := &tool.PipelineContext{
		RequestID: requestID,
		OriginalRequest: map[string]interface{}{
			"audio":                 audioData,
			"audio_format":          audioFormat,
			"audio_original_format": req.AudioFormat,
			"source_language":       req.SourceLanguage,
			"target_languages":      targetLangCodes,
			"task":                  string(req.Task),
			"options":               req.Options,
		},
		Dictionary: dictionary,
	}

	outCtx, err := p.pipelineExec.Execute(ctx, selected, pctx)
	if err != nil {
		return nil, err
	}

	resp, asrLanguage, err := p.buildPipelineResponse(req, selected, outCtx, audioFormat, conversionApplied)
	if err != nil {
		return nil, err
	}

	sourceLangForMetrics := req.SourceLanguage
	if sourceLangForMetrics == "" {
		sourceLangForMetrics = asrLanguage
	}

	if req.Task == prompt.TaskTranscribe && resp.Transcription != "" {
		metrics.IncTranscription(sourceLangForMetrics)
	}
	if req.Task == prompt.TaskTranslate {
		for code := range resp.Translations {
			metrics.IncTranslation(sourceLangForMetrics, code)
		}
	}

	return resp, nil
}

func (p *Processor) buildPipelineResponse(
	req ProcessRequest,
	selected pipeline.Pipeline,
	outCtx *tool.PipelineContext,
	processedFormat string,
	conversionApplied bool,
) (*ProcessResponse, string, error) {
	asrOut := outCtx.StepOutputs["asr_result"].Data
	transcription, _ := asrOut["text"].(string)
	asrLanguage, _ := asrOut["language"].(string)

	resp := acquireProcessResponse()
	resp.RequestID = generateRequestID()
	resp.Status = "success"
	resp.Transcription = transcription
	resp.Metadata["pipeline"] = selected.Name
	resp.Metadata["asr_language"] = asrLanguage
	resp.Metadata["asr_duration_ms"] = outCtx.Metrics["asr_result"].Milliseconds()
	resp.Metadata["original_format"] = req.AudioFormat
	resp.Metadata["processed_format"] = processedFormat
	resp.Metadata["conversion_applied"] = conversionApplied

	stepDurations := make(map[string]int64)
	for k, d := range outCtx.Metrics {
		stepDurations[k] = d.Milliseconds()
	}
	resp.Metadata["step_durations_ms"] = stepDurations

	if p.logger != nil {
		p.logger.WithFields(logrus.Fields{
			"pipeline":              selected.Name,
			"response_request_id":   resp.RequestID,
			"asr_language":          asrLanguage,
			"transcription_preview": previewResponseText(transcription),
		}).Debug("Pipeline transcription prepared")
	}

	switch selected.Name {
	case pipeline.PipelineTranscribe:
		// ASR only.
	case pipeline.PipelineTranscribeCorrect:
		correctOut := outCtx.StepOutputs["correct_result"]
		if v, ok := correctOut.Data["corrected_text"].(string); ok {
			resp.CorrectedText = v
		}
		if v, ok := correctOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		for k, v := range correctOut.Metadata {
			resp.Metadata[k] = v
		}
	case pipeline.PipelineTranslateMerged:
		ctOut := outCtx.StepOutputs["correct_translate_result"]
		if v, ok := ctOut.Data["corrected_text"].(string); ok {
			resp.CorrectedText = v
		}
		if v, ok := ctOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		if translations, ok := ctOut.Data["translations"].(map[string]string); ok {
			for k, v := range translations {
				resp.Translations[k] = v
			}
		}
		for k, v := range ctOut.Metadata {
			resp.Metadata[k] = v
		}
	case pipeline.PipelineTranslate:
		trOut := outCtx.StepOutputs["translate_result"]
		if v, ok := trOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		if translations, ok := trOut.Data["translations"].(map[string]string); ok {
			for k, v := range translations {
				resp.Translations[k] = v
			}
		}
		for k, v := range trOut.Metadata {
			resp.Metadata[k] = v
		}
	case pipeline.PipelineTranslateSplit:
		correctOut := outCtx.StepOutputs["correct_result"]
		translateOut := outCtx.StepOutputs["translate_result"]

		if v, ok := correctOut.Data["corrected_text"].(string); ok {
			resp.CorrectedText = v
		}
		if v, ok := translateOut.Data["raw_response"].(string); ok {
			resp.RawResponse = v
		}
		if translations, ok := translateOut.Data["translations"].(map[string]string); ok {
			for k, v := range translations {
				resp.Translations[k] = v
			}
		}

		resp.Metadata["correction_backend"] = correctOut.Metadata["backend"]
		resp.Metadata["translation_backend"] = translateOut.Metadata["backend"]
		resp.Metadata["raw_correction_response"] = correctOut.Data["raw_response"]

		// Keep "backend"/token fields aligned with the translation stage.
		for k, v := range translateOut.Metadata {
			resp.Metadata[k] = v
		}
	default:
		return nil, "", coreerrors.NewInternalError(fmt.Sprintf("unknown pipeline: %s", selected.Name), nil)
	}

	return resp, asrLanguage, nil
}

func previewResponseText(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 240 {
		return s
	}
	return s[:240] + "...(truncated)"
}
