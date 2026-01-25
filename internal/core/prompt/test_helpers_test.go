package prompt

import (
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/config"
)

func newTestPromptConfig() config.PromptConfig {
	return config.PromptConfig{
		Defaults: config.PromptDefaults{
			Task:            "translate",
			TargetLanguages: []string{"en", "ja"},
		},
		Languages: []config.Language{
			{
				Code: "zh",
				Names: map[string]string{
					"display": "中文",
					"english": "Chinese",
					"native":  "中文",
				},
				Aliases: []string{"chinese", "中文", "汉语", "zh-cn"},
			},
			{
				Code: "en",
				Names: map[string]string{
					"display": "英文",
					"english": "English",
					"native":  "English",
				},
				Aliases: []string{"english", "英文", "英语"},
			},
			{
				Code: "ja",
				Names: map[string]string{
					"display": "日文",
					"english": "Japanese",
					"native":  "日本語",
				},
				Aliases: []string{"japanese", "日文", "日语", "日本語"},
			},
		},
	}
}
