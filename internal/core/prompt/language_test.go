package prompt

import (
	"testing"

	"github.com/Lingualink-VRChat/Lingualink_Core/internal/testutil"
)

func TestLanguageManager_GetLanguage(t *testing.T) {
	t.Parallel()

	lm := NewLanguageManager(newTestPromptConfig(), testutil.NewTestLogger())

	if _, ok := lm.GetLanguage("zh"); !ok {
		t.Fatalf("expected zh to exist")
	}
	if _, ok := lm.GetLanguage("en"); !ok {
		t.Fatalf("expected en to exist")
	}
	if _, ok := lm.GetLanguage("ja"); !ok {
		t.Fatalf("expected ja to exist")
	}
}

func TestLanguageManager_NormalizeLanguage(t *testing.T) {
	t.Parallel()

	lm := NewLanguageManager(newTestPromptConfig(), testutil.NewTestLogger())

	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "zh", want: "zh"},
		{in: "ZH", want: "zh"},
		{in: "chinese", want: "zh"},
		{in: "中文", want: "zh"},
		{in: "英文", want: "en"},      // display name
		{in: "English", want: "en"}, // english name is in Aliases; display name check is also case-insensitive
		{in: "not-exist", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := lm.NormalizeLanguage(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeLanguage: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got=%q want %q", got, tt.want)
			}
		})
	}
}

func TestLanguageManager_ConvertCodesToDisplayNames(t *testing.T) {
	t.Parallel()

	lm := NewLanguageManager(newTestPromptConfig(), testutil.NewTestLogger())

	names, err := lm.ConvertCodesToDisplayNames([]string{"EN", "ja"})
	if err != nil {
		t.Fatalf("ConvertCodesToDisplayNames: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("len=%d want 2", len(names))
	}
	if names[0] != "英文" || names[1] != "日文" {
		t.Fatalf("names=%v want [英文 日文]", names)
	}
}

func TestLanguageManager_BuildDynamicOutputRules(t *testing.T) {
	t.Parallel()

	lm := NewLanguageManager(newTestPromptConfig(), testutil.NewTestLogger())

	rules := lm.BuildDynamicOutputRules(TaskTranslate, []string{"en", "ja"}, true)
	if rules.Format != FormatStructured {
		t.Fatalf("format=%q want %q", rules.Format, FormatStructured)
	}
	if rules.Separator == "" {
		t.Fatalf("separator is empty")
	}
	if len(rules.Sections) != 3 {
		t.Fatalf("sections=%d want 3", len(rules.Sections))
	}
	if rules.Sections[0].Key != "原文" || !rules.Sections[0].Required || rules.Sections[0].Order != 1 {
		t.Fatalf("unexpected source section: %+v", rules.Sections[0])
	}
	if rules.Sections[1].LanguageCode != "en" || rules.Sections[1].Order != 2 {
		t.Fatalf("unexpected en section: %+v", rules.Sections[1])
	}
	if rules.Sections[2].LanguageCode != "ja" || rules.Sections[2].Order != 3 {
		t.Fatalf("unexpected ja section: %+v", rules.Sections[2])
	}
}

func TestLanguageManager_IdentifyLanguageFromText(t *testing.T) {
	t.Parallel()

	lm := NewLanguageManager(newTestPromptConfig(), testutil.NewTestLogger())

	code, err := lm.IdentifyLanguageFromText("日本語")
	if err != nil {
		t.Fatalf("IdentifyLanguageFromText: %v", err)
	}
	if code != "ja" {
		t.Fatalf("code=%q want ja", code)
	}
}
