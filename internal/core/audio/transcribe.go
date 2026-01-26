// transcribe.go contains a helper ASR transcription method (legacy / diagnostic usage).
package audio

import (
	"context"
	"time"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/asr"
	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
)

func (p *Processor) transcribe(ctx context.Context, req ProcessRequest) (*asr.ASRResponse, string, bool, time.Duration, error) {
	if p.asrManager == nil {
		return nil, "", false, 0, coreerrors.NewInternalError("asr manager not configured", nil)
	}

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
		}
	}

	start := time.Now()
	asrResp, err := p.asrManager.Transcribe(ctx, &asr.ASRRequest{
		Audio:       audioData,
		AudioFormat: audioFormat,
		Language:    req.SourceLanguage,
	})
	if err != nil {
		return nil, audioFormat, conversionApplied, time.Since(start), err
	}
	return asrResp, audioFormat, conversionApplied, time.Since(start), nil
}
