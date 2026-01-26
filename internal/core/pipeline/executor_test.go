package pipeline

import (
	"context"
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/tool"
)

type mockTool struct {
	name     string
	validate func(tool.Input) error
	execute  func(context.Context, tool.Input) (tool.Output, error)
}

func (m mockTool) Name() string                         { return m.name }
func (m mockTool) Description() string                  { return m.name }
func (m mockTool) Schema() map[string]interface{}       { return nil }
func (m mockTool) OutputSchema() map[string]interface{} { return nil }
func (m mockTool) Validate(in tool.Input) error {
	if m.validate != nil {
		return m.validate(in)
	}
	return nil
}
func (m mockTool) Execute(ctx context.Context, in tool.Input) (tool.Output, error) {
	if m.execute != nil {
		return m.execute(ctx, in)
	}
	return tool.Output{}, nil
}

func TestExecutor_Execute_TranslateSplit(t *testing.T) {
	t.Parallel()

	reg := tool.NewRegistry()
	if err := reg.Register(mockTool{
		name: "asr",
		execute: func(ctx context.Context, in tool.Input) (tool.Output, error) {
			return tool.Output{Data: map[string]interface{}{"text": "hello", "language": "zh"}}, nil
		},
	}); err != nil {
		t.Fatalf("register asr: %v", err)
	}
	if err := reg.Register(mockTool{
		name: "correct",
		execute: func(ctx context.Context, in tool.Input) (tool.Output, error) {
			return tool.Output{Data: map[string]interface{}{"corrected_text": in.Data["text"].(string) + "!"}}, nil
		},
	}); err != nil {
		t.Fatalf("register correct: %v", err)
	}
	if err := reg.Register(mockTool{
		name: "translate",
		execute: func(ctx context.Context, in tool.Input) (tool.Output, error) {
			return tool.Output{Data: map[string]interface{}{"translations": map[string]string{"en": "hello"}}}, nil
		},
	}); err != nil {
		t.Fatalf("register translate: %v", err)
	}

	exec := NewExecutor(reg)
	pctx := &tool.PipelineContext{
		OriginalRequest: map[string]interface{}{
			"audio":            []byte{0x00},
			"audio_format":     "wav",
			"source_language":  "",
			"target_languages": []string{"en"},
		},
	}

	outCtx, err := exec.Execute(context.Background(), TranslateSplit(), pctx)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	asrOut := outCtx.StepOutputs["asr_result"].Data
	if got := asrOut["text"]; got != "hello" {
		t.Fatalf("asr_result.text=%v want hello", got)
	}

	correctOut := outCtx.StepOutputs["correct_result"].Data
	if got := correctOut["corrected_text"]; got != "hello!" {
		t.Fatalf("correct_result.corrected_text=%v want hello!", got)
	}

	translateOut := outCtx.StepOutputs["translate_result"].Data
	translations, ok := translateOut["translations"].(map[string]string)
	if !ok {
		t.Fatalf("translations type=%T", translateOut["translations"])
	}
	if translations["en"] != "hello" {
		t.Fatalf("en=%q want hello", translations["en"])
	}
}

func TestExecutor_Execute_MissingMapping(t *testing.T) {
	t.Parallel()

	reg := tool.NewRegistry()
	_ = reg.Register(mockTool{name: "noop"})

	exec := NewExecutor(reg)
	pctx := &tool.PipelineContext{OriginalRequest: map[string]interface{}{}}

	_, err := exec.Execute(context.Background(), Pipeline{
		Name: "bad",
		Steps: []Step{
			{
				ToolName: "noop",
				InputMapping: map[string]string{
					"x": "request.nope",
				},
				OutputKey: "out",
			},
		},
	}, pctx)
	if err == nil {
		t.Fatalf("expected error")
	}
}
