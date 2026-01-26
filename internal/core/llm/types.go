package llm

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ToolDefinition describes an OpenAI-compatible tool that can be provided to chat completions.
// See https://platform.openai.com/docs/guides/function-calling
type ToolDefinition struct {
	Type     string                 `json:"type"`
	Function ToolFunctionDefinition `json:"function"`
}

// ToolFunctionDefinition describes a function tool and its JSON schema parameters.
type ToolFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolChoiceMode matches OpenAI's tool_choice modes.
type ToolChoiceMode string

const (
	ToolChoiceNone     ToolChoiceMode = "none"
	ToolChoiceAuto     ToolChoiceMode = "auto"
	ToolChoiceRequired ToolChoiceMode = "required"
	ToolChoiceFunction ToolChoiceMode = "function"
)

// ToolChoice controls how the model should pick tools.
// It marshals into either a string ("auto"/"none"/"required") or an object
// ({"type":"function","function":{"name":"..."}}).
type ToolChoice struct {
	Mode         ToolChoiceMode
	FunctionName string
}

func (c ToolChoice) MarshalJSON() ([]byte, error) {
	switch c.Mode {
	case ToolChoiceNone, ToolChoiceAuto, ToolChoiceRequired:
		return json.Marshal(string(c.Mode))
	case ToolChoiceFunction:
		if c.FunctionName == "" {
			return nil, errors.New("tool_choice function name is required")
		}
		return json.Marshal(map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name": c.FunctionName,
			},
		})
	case "":
		return []byte("null"), nil
	default:
		return nil, fmt.Errorf("unknown tool_choice mode: %q", c.Mode)
	}
}

// ToolCall is a single tool call emitted by the model.
type ToolCall struct {
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function"`
}

// ToolCallFunction contains tool call function payload.
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ParseToolCallResponse finds a matching tool call and unmarshals its arguments into out.
func ParseToolCallResponse(resp *LLMResponse, functionName string, out any) error {
	if resp == nil {
		return errors.New("nil LLM response")
	}
	if len(resp.ToolCalls) == 0 {
		return errors.New("no tool calls in response")
	}
	for _, call := range resp.ToolCalls {
		if call.Function.Name != functionName {
			continue
		}
		if call.Function.Arguments == "" {
			return fmt.Errorf("tool call %q has empty arguments", functionName)
		}
		if err := json.Unmarshal([]byte(call.Function.Arguments), out); err != nil {
			return fmt.Errorf("unmarshal tool call arguments: %w", err)
		}
		return nil
	}
	return fmt.Errorf("tool call %q not found", functionName)
}
