# RAG evaluation: what's covered, and the real gap

Four evaluation/monitoring areas are implemented (`internal/metrics`,
`integration_tests/`, Prometheus + Grafana):

1. **Retrieval quality**: `integration_tests/retrieval_eval_test.go` maps a
   golden query to expected product titles, scored as recall@FinalTopN
   against the real vector+fulltext+graph+RRF pipeline.
2. **Intent classification accuracy**: `integration_tests/intent_eval_test.go`
   maps a golden query to an expected intent, scored as accuracy against the
   real Gemini classifier.
3. **Citation/hallucination monitoring**: `citation_outcomes_total{outcome}`
   in `internal/llm/gemini/client.go`'s `GenerateAnswer` counts how often a
   self-reported `cited_product_id` is valid versus hallucinated.
4. **Per-stage latency, signal contribution, and token/cost**:
   `pipeline_stage_duration_seconds`, `retrieval_signal_contribution_total`,
   and `gemini_tokens_total` in `internal/pipeline/query_pipeline.go` and
   `internal/llm/gemini/client.go`, scraped by Prometheus and visualized in
   the provisioned Grafana dashboard (`grafana/dashboards/product-rag-search.json`).

None of these four measure the faithfulness or correctness of the generated
answer's content. Citation monitoring (#3) checks only that a cited product
ID is real; it does not check whether the answer's claims about that
product are true. Retrieval quality (#1) checks only that the pipeline
fetched the right products, not what the model said about them. Measuring
faithfulness needs an LLM-as-judge step: feed the answer and the retrieved
product docs to a judge model, and score whether every claim is grounded.
That step is scoped but not built. Two options if it becomes worth
building:

- **Ragas (Python)**: mature, standardized metrics (faithfulness via claim
  decomposition, answer relevancy, context precision/recall). It is a
  Python library, not a service, so adopting it means a new one-shot batch
  container in docker-compose (same pattern as the `seed` service), a
  `requirements.txt`, and a separate Gemini integration path via LiteLLM
  (`gemini/<model>`) or Vertex AI. It does not reuse the Go `google-genai`
  client already in this repo.
- **Hand-rolled Go judge**: one more method on `internal/llm/gemini`, same
  structured-output pattern as `Classify`/`GenerateAnswer`. Feed it
  `(query, answer, retrieved product docs)` and get back a faithfulness
  score and a list of unsupported claims. No new language, no new
  container, reuses the existing client and API key. The tradeoff: the
  judge prompt is hand-written and untested against Ragas's corpus of prior
  use.

Everything else in this repo favors single-language, low-dependency
choices, so the hand-rolled option fits the pattern better. It trades
rigor for simplicity, and that tradeoff is not a clear-cut call.
