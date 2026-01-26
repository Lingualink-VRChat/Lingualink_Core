package prompt

import (
	"testing"
)

func TestExtractJSONBlock(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantOK  bool
		wantRaw string
	}{
		{
			name:    "valid_json_block",
			raw:     "prefix\n```json\n{\"a\":1}\n```\nsuffix",
			wantOK:  true,
			wantRaw: "{\"a\":1}",
		},
		{
			name:    "missing_json_marker",
			raw:     "```txt\n{\"a\":1}\n```",
			wantOK:  false,
			wantRaw: "",
		},
		{
			name:    "no_block",
			raw:     "{\"a\":1}",
			wantOK:  false,
			wantRaw: "",
		},
		{
			name:    "multiple_blocks_first_wins",
			raw:     "```json\n{\"first\":true}\n```\n...\n```json\n{\"second\":true}\n```",
			wantOK:  true,
			wantRaw: "{\"first\":true}",
		},
		{
			name:    "empty_object",
			raw:     "```json\n{}\n```",
			wantOK:  true,
			wantRaw: "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractJSONBlock(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok=%v want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if string(got) != tt.wantRaw {
				t.Fatalf("got=%q want %q", string(got), tt.wantRaw)
			}
		})
	}
}

func TestParseJSONResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		jsonData     string
		wantTrans    string
		wantSource   string
		wantCorrect  string
		wantSections map[string]string
		wantErr      bool
	}{
		{
			name:      "transcription_and_translations",
			jsonData:  `{"transcription":"你好","translations":{"en":"hello","ja":"こんにちは"}}`,
			wantTrans: "你好",
			wantSections: map[string]string{
				"en": "hello",
				"ja": "こんにちは",
			},
		},
		{
			name:       "source_text_overwrites_transcription",
			jsonData:   `{"transcription":"A","source_text":"B","translations":{"en":"C"}}`,
			wantTrans:  "A",
			wantSource: "B",
			wantSections: map[string]string{
				"en": "C",
			},
		},
		{
			name:       "unicode_and_escaping",
			jsonData:   `{"source_text":"含有\n换行","translations":{"en":"He said: \"hi\""}}`,
			wantSource: "含有\n换行",
			wantSections: map[string]string{
				"en": "He said: \"hi\"",
			},
		},
		{
			name:        "corrected_text",
			jsonData:    `{"corrected_text":"你好！","translations":{"en":"hello"}}`,
			wantCorrect: "你好！",
			wantSections: map[string]string{
				"en": "hello",
			},
		},
		{
			name:     "invalid_json",
			jsonData: `{"source_text":`,
			wantErr:  true,
		},
		{
			name:       "nested_extra_fields_ignored",
			jsonData:   `{"source_text":"x","translations":{"en":"y"},"extra":{"a":1}}`,
			wantSource: "x",
			wantSections: map[string]string{
				"en": "y",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseJSONResponse([]byte(tt.jsonData))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseJSONResponse: %v", err)
			}
			if parsed.Transcription != tt.wantTrans {
				t.Fatalf("Transcription=%q want %q", parsed.Transcription, tt.wantTrans)
			}
			if parsed.SourceText != tt.wantSource {
				t.Fatalf("SourceText=%q want %q", parsed.SourceText, tt.wantSource)
			}
			if parsed.CorrectedText != tt.wantCorrect {
				t.Fatalf("CorrectedText=%q want %q", parsed.CorrectedText, tt.wantCorrect)
			}
			for k, want := range tt.wantSections {
				if got := parsed.Sections[k]; got != want {
					t.Fatalf("section[%q]=%q want %q", k, got, want)
				}
			}
			if parsed.Metadata["parser"] != "json" {
				t.Fatalf("parser=%v want json", parsed.Metadata["parser"])
			}
			if parsed.Metadata["parse_success"] != true {
				t.Fatalf("parse_success=%v want true", parsed.Metadata["parse_success"])
			}
		})
	}
}
