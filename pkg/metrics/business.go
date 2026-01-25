package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	translationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lingualink_translations_total",
			Help: "Total successful translations produced",
		},
		[]string{"source", "target"},
	)

	transcriptionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lingualink_transcriptions_total",
			Help: "Total successful transcriptions produced",
		},
		[]string{"source"},
	)

	languagePairUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "lingualink_language_pair_usage_total",
			Help: "Total usage of language pairs for translation",
		},
		[]string{"source", "target"},
	)

	jsonParseSuccessRate = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lingualink_json_parse_success_rate",
			Help: "JSON parse success indicator (1=success, 0=failure), suitable for calculating success rate over time",
		},
		[]string{"parser"},
	)
)

// IncTranslation records a successful translation for a given language pair.
func IncTranslation(sourceLang, targetLang string) {
	if sourceLang == "" {
		sourceLang = "auto"
	}
	if targetLang == "" {
		targetLang = "unknown"
	}
	translationsTotal.WithLabelValues(sourceLang, targetLang).Inc()
	languagePairUsage.WithLabelValues(sourceLang, targetLang).Inc()
}

// IncTranscription records a successful transcription.
func IncTranscription(sourceLang string) {
	if sourceLang == "" {
		sourceLang = "auto"
	}
	transcriptionsTotal.WithLabelValues(sourceLang).Inc()
}

// ObserveJSONParseSuccess records whether a JSON parse succeeded.
// The gauge value is 1 for success, 0 for failure.
func ObserveJSONParseSuccess(parser string, success bool) {
	if parser == "" {
		parser = "json"
	}
	if success {
		jsonParseSuccessRate.WithLabelValues(parser).Set(1)
		return
	}
	jsonParseSuccessRate.WithLabelValues(parser).Set(0)
}
