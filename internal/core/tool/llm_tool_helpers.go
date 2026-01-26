package tool

import (
	"strings"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/llm"
)

const submitResultFunctionName = "submit_result"

func submitResultTools(outputSchema map[string]interface{}, description string) []llm.ToolDefinition {
	if outputSchema == nil {
		return nil
	}
	return []llm.ToolDefinition{
		{
			Type: "function",
			Function: llm.ToolFunctionDefinition{
				Name:        submitResultFunctionName,
				Description: description,
				Parameters:  outputSchema,
			},
		},
	}
}

func bestEffortRawResponse(resp *llm.LLMResponse) string {
	if resp == nil {
		return ""
	}
	if strings.TrimSpace(resp.Content) != "" {
		return resp.Content
	}
	if len(resp.ToolCalls) > 0 {
		if args := strings.TrimSpace(resp.ToolCalls[0].Function.Arguments); args != "" {
			return args
		}
	}
	return ""
}

func coerceStringSlice(v interface{}) ([]string, bool) {
	switch vv := v.(type) {
	case []string:
		out := make([]string, 0, len(vv))
		for _, s := range vv {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, true
	case []interface{}:
		out := make([]string, 0, len(vv))
		for _, item := range vv {
			s, ok := item.(string)
			if !ok {
				continue
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, true
	default:
		return nil, false
	}
}
