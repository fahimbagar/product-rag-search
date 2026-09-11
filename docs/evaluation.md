# RAG evaluation: what's covered, and the real gap

Four evaluation/monitoring areas are implemented (`internal/metrics`,
`eval/integration_tests/`, Prometheus + Grafana):

1. **Retrieval quality** — `eval/integration_tests/retrieval_eval_test.go`,
   golden query → expected product titles, scored as recall@FinalTopN
   against the real vector+fulltext+graph+RRF pipeline.
2. **Intent classification accuracy** — `eval/integration_tests/intent_eval_test.go`,
   golden query → expected intent, scored as accuracy against the real
   Gemini classifier.
3. **Citation/hallucination monitoring** — `citation_outcomes_total{outcome}`
   in `internal/llm/gemini/client.go`'s `GenerateAnswer`, counting how often
   a self-reported `cited_product_id` is valid vs. hallucinated.
4. **Per-stage latency, signal contribution, and token/cost** —
   `pipeline_stage_duration_seconds`, `retrieval_signal_contribution_total`,
   and `gemini_tokens_total` in `internal/pipeline/query_pipeline.go` and
   `internal/llm/gemini/client.go`, scraped by Prometheus and visualized in
   the provisioned Grafana dashboard (`grafana/dashboards/product-rag-search.json`).

**What none of these four cover: faithfulness and correctness of the
generated answer's *content*.** Citation monitoring (#3) only checks that a
cited product ID is real — it does not check whether the answer's claims
about that product are actually true. Retrieval quality (#1) only checks
that the right products were fetched, not what the model said about them.
Measuring this properly needs an LLM-as-judge step (feed the answer +
retrieved product docs to a judge model, score whether every claim is
grounded), which was scoped and deliberately deferred — see the two options
below if it becomes worth building:

- **Ragas (Python)** — mature, standardized metrics (faithfulness via claim
  decomposition, answer relevancy, context precision/recall), but it's a
  Python library, not a service: adopting it means a new one-shot batch
  container in docker-compose (same pattern as the `seed` service), a
  `requirements.txt`, and a separate Gemini integration path via LiteLLM
  (`gemini/<model>`) or Vertex AI — it doesn't reuse the Go `google-genai`
  client already in this repo.
- **Hand-rolled Go judge** — one more method on `internal/llm/gemini`, same
  structured-output pattern as `Classify`/`GenerateAnswer`: feed it
  `(query, answer, retrieved product docs)`, get back a faithfulness score
  and a list of unsupported claims. No new language, no new container,
  reuses the existing client and API key — but the judge prompt is
  hand-written and unvalidated against a large existing corpus of use,
  unlike Ragas's.

Given everything else in this repo favors single-language/low-dependency
choices, the hand-rolled option fits the pattern better — but it's a real
tradeoff (rigor vs. simplicity), not a clear-cut call.
