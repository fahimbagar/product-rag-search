// Package metrics defines the Prometheus metrics emitted across the
// service: HTTP-level request metrics, per-stage pipeline timings, retrieval
// signal contribution, citation outcomes, Gemini token usage, and intent
// classification counts. Scraped via GET /metrics.
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests handled, by method, path, and status.",
	}, []string{"method", "path", "status"})

	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by method and path.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	// PipelineStageDuration times each stage of the /query pipeline:
	// classify, embed, vector_search, fulltext_search, graph_search,
	// rerank, hydrate, generate, deflect.
	PipelineStageDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "pipeline_stage_duration_seconds",
		Help:    "Duration of each RAG pipeline stage in seconds, by stage.",
		Buckets: prometheus.DefBuckets,
	}, []string{"stage"})

	// RetrievalSignalContribution counts, per final top-N result, which
	// retrieval source(s) (vector, fulltext, graph) surfaced it. A result
	// found by multiple signals increments each of them.
	RetrievalSignalContribution = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "retrieval_signal_contribution_total",
		Help: "Count of final top-N results each retrieval signal contributed to, by source.",
	}, []string{"source"})

	// CitationOutcomes counts self-reported cited_product_ids from
	// GenerateAnswer as valid (a real candidate) or hallucinated (not
	// among the candidates offered).
	CitationOutcomes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "citation_outcomes_total",
		Help: "Self-reported product citations from generation, by outcome (valid, hallucinated).",
	}, []string{"outcome"})

	// GeminiTokensTotal tracks token usage per Gemini call, by call kind
	// (classify, generate, deflect) and token kind (prompt, candidates, total).
	GeminiTokensTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "gemini_tokens_total",
		Help: "Gemini API token usage, by call and token kind.",
	}, []string{"call", "kind"})

	// IntentClassificationsTotal counts classified queries by intent label.
	IntentClassificationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "intent_classifications_total",
		Help: "Count of classified queries, by intent label.",
	}, []string{"intent"})
)
