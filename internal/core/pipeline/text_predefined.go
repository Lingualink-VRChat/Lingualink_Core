package pipeline

const (
	PipelineTextTranslate            = "text_translate"
	PipelineTextCorrect              = "text_correct"
	PipelineTextCorrectTranslate     = "text_correct_translate"
	PipelineTextCorrectThenTranslate = "text_correct_then_translate"
	PipelineTextPassthrough          = "text_passthrough"
)

func TextTranslate() Pipeline {
	return Pipeline{
		Name: PipelineTextTranslate,
		Steps: []Step{
			{
				ToolName: "text_translate",
				InputMapping: map[string]string{
					"text":             "request.text",
					"source_language":  "request.source_language",
					"target_languages": "request.target_languages",
				},
				OutputKey: "translate_result",
			},
		},
	}
}

func TextCorrect() Pipeline {
	return Pipeline{
		Name: PipelineTextCorrect,
		Steps: []Step{
			{
				ToolName: "text_correct",
				InputMapping: map[string]string{
					"text": "request.text",
				},
				OutputKey: "correct_result",
			},
		},
	}
}

func TextCorrectTranslate() Pipeline {
	return Pipeline{
		Name: PipelineTextCorrectTranslate,
		Steps: []Step{
			{
				ToolName: "text_correct_translate",
				InputMapping: map[string]string{
					"text":             "request.text",
					"target_languages": "request.target_languages",
				},
				OutputKey: "correct_translate_result",
			},
		},
	}
}

func TextCorrectThenTranslate() Pipeline {
	return Pipeline{
		Name: PipelineTextCorrectThenTranslate,
		Steps: []Step{
			{
				ToolName: "text_correct",
				InputMapping: map[string]string{
					"text": "request.text",
				},
				OutputKey: "correct_result",
			},
			{
				ToolName: "text_translate",
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

func TextPassthrough() Pipeline {
	return Pipeline{
		Name:  PipelineTextPassthrough,
		Steps: nil,
	}
}
