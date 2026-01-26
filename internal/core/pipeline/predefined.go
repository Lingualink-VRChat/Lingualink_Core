package pipeline

const (
	PipelineTranscribe        = "transcribe"
	PipelineTranscribeCorrect = "transcribe_correct"
	PipelineTranslateMerged   = "translate_merged"
	PipelineTranslateSplit    = "translate_split"
	PipelineTranslate         = "translate"
)

func Transcribe() Pipeline {
	return Pipeline{
		Name: PipelineTranscribe,
		Steps: []Step{
			{
				ToolName: "asr",
				InputMapping: map[string]string{
					"audio":    "request.audio",
					"format":   "request.audio_format",
					"language": "request.source_language",
				},
				OutputKey: "asr_result",
			},
		},
	}
}

func TranscribeCorrect() Pipeline {
	return Pipeline{
		Name: PipelineTranscribeCorrect,
		Steps: []Step{
			{
				ToolName: "asr",
				InputMapping: map[string]string{
					"audio":    "request.audio",
					"format":   "request.audio_format",
					"language": "request.source_language",
				},
				OutputKey: "asr_result",
			},
			{
				ToolName: "correct",
				InputMapping: map[string]string{
					"text": "asr_result.text",
				},
				OutputKey: "correct_result",
			},
		},
	}
}

func TranslateMerged() Pipeline {
	return Pipeline{
		Name: PipelineTranslateMerged,
		Steps: []Step{
			{
				ToolName: "asr",
				InputMapping: map[string]string{
					"audio":    "request.audio",
					"format":   "request.audio_format",
					"language": "request.source_language",
				},
				OutputKey: "asr_result",
			},
			{
				ToolName: "correct_translate",
				InputMapping: map[string]string{
					"text":             "asr_result.text",
					"target_languages": "request.target_languages",
				},
				OutputKey: "correct_translate_result",
			},
		},
	}
}

func TranslateSplit() Pipeline {
	return Pipeline{
		Name: PipelineTranslateSplit,
		Steps: []Step{
			{
				ToolName: "asr",
				InputMapping: map[string]string{
					"audio":    "request.audio",
					"format":   "request.audio_format",
					"language": "request.source_language",
				},
				OutputKey: "asr_result",
			},
			{
				ToolName: "correct",
				InputMapping: map[string]string{
					"text": "asr_result.text",
				},
				OutputKey: "correct_result",
			},
			{
				ToolName: "translate",
				InputMapping: map[string]string{
					"text":             "correct_result.corrected_text",
					"source_language":  "request.source_language",
					"target_languages": "request.target_languages",
				},
				OutputKey: "translate_result",
			},
		},
	}
}

func Translate() Pipeline {
	return Pipeline{
		Name: PipelineTranslate,
		Steps: []Step{
			{
				ToolName: "asr",
				InputMapping: map[string]string{
					"audio":    "request.audio",
					"format":   "request.audio_format",
					"language": "request.source_language",
				},
				OutputKey: "asr_result",
			},
			{
				ToolName: "translate",
				InputMapping: map[string]string{
					"text":             "asr_result.text",
					"source_language":  "request.source_language",
					"target_languages": "request.target_languages",
				},
				OutputKey: "translate_result",
			},
		},
	}
}
