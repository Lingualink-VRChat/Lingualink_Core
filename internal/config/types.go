package config

// Config defines the full runtime configuration for Lingualink Core.
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Auth       AuthConfig       `mapstructure:"auth"`
	ASR        ASRConfig        `mapstructure:"asr"`
	Correction CorrectionConfig `mapstructure:"correction"`
	Pipeline   PipelineConfig   `mapstructure:"pipeline"`
	Backends   BackendsConfig   `mapstructure:"backends"`
	Prompt     PromptConfig     `mapstructure:"prompt"`
	Logging    LoggingConfig    `mapstructure:"logging"`
}

// ServerConfig controls the HTTP server.
type ServerConfig struct {
	Mode string `mapstructure:"mode"`
	Port int    `mapstructure:"port"`
	Host string `mapstructure:"host"`
}

// AuthConfig configures authentication strategies.
type AuthConfig struct {
	Strategies []AuthStrategy `mapstructure:"strategies"`
}

// AuthStrategy represents one authentication strategy configuration entry.
type AuthStrategy struct {
	Type     string                 `mapstructure:"type"`
	Enabled  bool                   `mapstructure:"enabled"`
	Config   map[string]interface{} `mapstructure:"config"`
	Endpoint string                 `mapstructure:"endpoint"`
}

// ASRConfig configures ASR providers.
type ASRConfig struct {
	Providers []ASRProvider `mapstructure:"providers"`
}

// ASRProvider configures an ASR backend provider.
type ASRProvider struct {
	Name       string                 `mapstructure:"name"`
	Type       string                 `mapstructure:"type"` // whisper / sensevoice / custom
	URL        string                 `mapstructure:"url"`
	Model      string                 `mapstructure:"model"`
	APIKey     string                 `mapstructure:"api_key"`
	Parameters map[string]interface{} `mapstructure:"parameters"`
}

// CorrectionConfig configures the optional correction stage.
type CorrectionConfig struct {
	Enabled              bool             `mapstructure:"enabled"`
	MergeWithTranslation bool             `mapstructure:"merge_with_translation"`
	GlobalDictionary     []DictionaryTerm `mapstructure:"global_dictionary"`
}

// DictionaryTerm represents a terminology mapping used during correction.
type DictionaryTerm struct {
	Term    string   `mapstructure:"term"`
	Aliases []string `mapstructure:"aliases"`
}

// BackendsConfig configures LLM backend providers and load balancing.
type BackendsConfig struct {
	LoadBalancer LoadBalancerConfig `mapstructure:"load_balancer"`
	Providers    []BackendProvider  `mapstructure:"providers"`
}

// LoadBalancerConfig configures the backend selection strategy.
type LoadBalancerConfig struct {
	Strategy string `mapstructure:"strategy"`
}

// BackendProvider defines one LLM provider instance.
type BackendProvider struct {
	Name       string                 `mapstructure:"name"`
	Type       string                 `mapstructure:"type"`
	Config     map[string]interface{} `mapstructure:"config"`
	URL        string                 `mapstructure:"url"`
	Model      string                 `mapstructure:"model"`
	APIKey     string                 `mapstructure:"api_key"`
	Parameters LLMParameters          `mapstructure:"parameters"`
}

// LLMParameters configures per-request/default model parameters.
type LLMParameters struct {
	Temperature       *float64 `mapstructure:"temperature"`
	MaxTokens         *int     `mapstructure:"max_tokens"`
	TopP              *float64 `mapstructure:"top_p"`
	TopK              *int     `mapstructure:"top_k"`
	RepetitionPenalty *float64 `mapstructure:"repetition_penalty"`
	FrequencyPenalty  *float64 `mapstructure:"frequency_penalty"`
	PresencePenalty   *float64 `mapstructure:"presence_penalty"`
	Stop              []string `mapstructure:"stop"`
	Seed              *int     `mapstructure:"seed"`
	Stream            *bool    `mapstructure:"stream"`
}

// PromptConfig controls prompt templates, languages, and parsing rules.
type PromptConfig struct {
	Defaults  PromptDefaults `mapstructure:"defaults"`
	Languages []Language     `mapstructure:"languages"`
	Parsing   ParsingConfig  `mapstructure:"parsing"`
}

// PromptDefaults sets default prompt behavior.
type PromptDefaults struct {
	Task            string   `mapstructure:"task"`
	TargetLanguages []string `mapstructure:"target_languages"`
}

// Language defines a supported language or style.
type Language struct {
	Code      string            `mapstructure:"code"`
	Type      string            `mapstructure:"type"` // standard | fun (default: standard)
	Names     map[string]string `mapstructure:"names"`
	Aliases   []string          `mapstructure:"aliases"`
	StyleNote string            `mapstructure:"style_note"` // Optional (usually for fun languages)
}

// ParsingConfig configures response parsing behavior.
type ParsingConfig struct {
	Separators []string         `mapstructure:"separators"`
	StrictMode bool             `mapstructure:"strict_mode"`
	Validation ValidationConfig `mapstructure:"validation"`
}

// ValidationConfig configures parsing validation rules.
type ValidationConfig struct {
	RequiredSections []string `mapstructure:"required_sections"`
	MinContentLength int      `mapstructure:"min_content_length"`
	MaxContentLength int      `mapstructure:"max_content_length"`
}

// LoggingConfig configures log output.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}
