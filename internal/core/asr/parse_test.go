package asr

import "testing"

func TestParseASRText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantLang string
		wantText string
	}{
		{
			name:     "wrapped chinese text",
			input:    "language Chinese<asr_text>你好世界",
			wantLang: "Chinese",
			wantText: "你好世界",
		},
		{
			name:     "wrapped english text",
			input:    "language English<asr_text>Hello World",
			wantLang: "English",
			wantText: "Hello World",
		},
		{
			name:     "none means empty text",
			input:    "language None<asr_text>",
			wantLang: "",
			wantText: "",
		},
		{
			name:     "plain text fallback",
			input:    "plain text",
			wantLang: "",
			wantText: "plain text",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotLang, gotText := ParseASRText(tt.input)
			if gotLang != tt.wantLang {
				t.Fatalf("language = %q, want %q", gotLang, tt.wantLang)
			}
			if gotText != tt.wantText {
				t.Fatalf("text = %q, want %q", gotText, tt.wantText)
			}
		})
	}
}
