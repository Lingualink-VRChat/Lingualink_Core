package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	coreerrors "github.com/Lingualink-VRChat/Lingualink_Core/internal/core/errors"
	"github.com/Lingualink-VRChat/Lingualink_Core/internal/core/tool"
)

type Executor struct {
	registry *tool.Registry
}

func NewExecutor(registry *tool.Registry) *Executor {
	return &Executor{registry: registry}
}

func (e *Executor) Execute(ctx context.Context, p Pipeline, pctx *tool.PipelineContext) (*tool.PipelineContext, error) {
	if e == nil || e.registry == nil {
		return nil, coreerrors.NewInternalError("pipeline executor not configured", nil)
	}
	if pctx == nil {
		pctx = &tool.PipelineContext{}
	}
	if pctx.StepOutputs == nil {
		pctx.StepOutputs = make(map[string]tool.Output)
	}
	if pctx.Metrics == nil {
		pctx.Metrics = make(map[string]time.Duration)
	}

	for _, step := range p.Steps {
		if strings.TrimSpace(step.ToolName) == "" {
			return nil, coreerrors.NewValidationError("pipeline step tool name is required", nil)
		}
		if strings.TrimSpace(step.OutputKey) == "" {
			return nil, coreerrors.NewValidationError("pipeline step output key is required", nil)
		}

		t, ok := e.registry.Get(step.ToolName)
		if !ok {
			return nil, coreerrors.NewValidationError(fmt.Sprintf("tool not registered: %s", step.ToolName), nil)
		}

		stepInput := make(map[string]interface{})
		for key, expr := range step.InputMapping {
			val, ok := resolveValue(pctx, expr)
			if !ok {
				return nil, coreerrors.NewValidationError(fmt.Sprintf("failed to resolve input mapping %q: %s", key, expr), nil)
			}
			stepInput[key] = val
		}

		in := tool.Input{Data: stepInput, Context: pctx}
		if err := t.Validate(in); err != nil {
			return nil, err
		}

		start := time.Now()
		out, err := t.Execute(ctx, in)
		pctx.Metrics[step.OutputKey] = time.Since(start)
		if err != nil {
			return nil, err
		}

		pctx.StepOutputs[step.OutputKey] = out
	}

	return pctx, nil
}

func resolveValue(pctx *tool.PipelineContext, expr string) (interface{}, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, false
	}

	parts := strings.Split(expr, ".")
	if len(parts) < 2 {
		return nil, false
	}

	var current interface{}
	switch parts[0] {
	case "request":
		current = pctx.OriginalRequest
	default:
		out, ok := pctx.StepOutputs[parts[0]]
		if !ok {
			return nil, false
		}
		current = out.Data
	}

	for _, key := range parts[1:] {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		v, ok := m[key]
		if !ok {
			return nil, false
		}
		current = v
	}

	return current, true
}
