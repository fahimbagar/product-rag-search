# Future improvements

## Alternative LLM / embeddings providers

Gemini (`gemini-3.5-flash-lite` for intent, `gemini-3.8-flash` for generation,
`gemini-embedding-001` for embeddings) is the current default — one provider,
one API key, a genuinely **permanent free tier** for both chat and embeddings
(verified against Google's pricing docs), unlike OpenAI/Anthropic's one-time
trial credits. Both seams are interfaces (`intent.Classifier`/`llm.Generator`
and `embeddings.Embedder`), so either can be swapped independently if cost
stops being the deciding factor.

**Claude — previously used here, and the real upgrade path for verified
citations:** this project originally used Claude (`claude-haiku-4-5` /
`claude-sonnet-5`) specifically for its `citations` API. Sending each
retrieved product as a `document` content block with
`citations: {enabled: true}` gets back a response where each answer segment
is linked to the exact source document and the exact quoted span of text
that supports it — the API computes and verifies this itself; the model
cannot fake a citation.

Gemini has no equivalent for documents supplied inline per-request. Its
closest feature, File Search, requires pre-indexing a whole corpus into a
persistent store and lets Gemini do its own retrieval internally — which
conflicts with this project's own hybrid retrieval/RRF pipeline (see the
package doc on `internal/llm/gemini/client.go` for the full reasoning). The
current workaround, implemented in that file's `GenerateAnswer`: a JSON
schema requiring `{"answer": "string", "cited_product_ids": ["string"]}`,
with the model self-reporting which products it used. We do validate the
reported IDs are actually among the candidates we gave it (rules out
hallucinated IDs that were never offered), but nothing verifies the *claim*
itself — that the answer text genuinely relied on that product. So the
`cited` flag on each product in the `/query` response is "the model's
self-report, filtered for validity," not "API-verified."

**If citation accuracy becomes important** (e.g. this moves from a demo
toward something where incorrect grounding has real consequences): swap
`internal/llm/gemini` for a `internal/llm/claude` implementation of the same
`llm.Generator` interface — nothing else in the pipeline needs to change,
that's the point of the interface. Budget for Claude's cost at that point
(no free tier, roughly $1-10 per million tokens depending on model).

(DeepSeek was also considered and ruled out: no free tier at all — it's
cheap, pay-as-you-go from the first token, not free — plus no embeddings API
and the same citation-verification gap as Gemini.)

## Alternative rerankers

The current reranker is Reciprocal Rank Fusion (`internal/rerank/rrf`) — a pure-Go
formula (`score = Σ 1/(k + rank)`) that merges the three retrieval signals'
rank *positions*, with no understanding of the query or document text itself.
It was chosen because it's free, has no extra infra, and is a well-established
technique (used in production by Elasticsearch/OpenSearch hybrid search), but
it has a real ceiling: two candidates ranked #1 by different signals for
completely different reasons score identically to two candidates that are
genuinely both excellent matches — RRF can't tell "coincidentally ranked
similarly" apart from "actually relevant."

`rerank.Reranker` is the interface (`internal/rerank/rerank.go`) precisely so a
better-quality reranker can be swapped in later without touching the
pipeline. The candidates, in increasing order of quality/cost:

**Hosted cross-encoder rerank APIs** — a cross-encoder jointly processes the
query and each candidate document's *text* (title/description/attributes),
not just its rank position, so it can actually judge relevance rather than
agreement between signals. Two managed options that fit our `Reranker`
interface as a drop-in (query + documents in, reranked scores out):
- **Voyage AI rerank** (`rerank-2.5` / `rerank-3`) — a different vendor than
  our current Gemini embeddings, but same "call an API, get scores back" shape.
- **Cohere Rerank** (`rerank-v4.0-pro` / `-fast`) — same shape, different
  vendor.

Both require an extra API key and a network call per query (added latency +
cost), but no extra infrastructure to run ourselves.

**Self-hosted cross-encoder** (e.g. `bge-reranker-v2-m3`) via a small
Python/ONNX sidecar container — avoids per-query API cost and an external
network hop, but adds a service to build, deploy, and keep patched. This is
the one option that conflicts with the original "simple for production, easy
to maintain" requirement that led to choosing RRF in the first place — only
worth it if RRF's relevance ceiling is measured to be a real problem *and*
the hosted APIs' cost/latency at volume rules them out too.

**LLM-based reranker** — feed the top-K candidates back to Claude/Gemini and
ask it to score or reorder them. Highest potential quality (full reasoning
over text, can weigh business rules), but highest latency/cost per query of
any option, and least deterministic.

**Recommendation if RRF turns out to be the bottleneck:** try a hosted
cross-encoder (Voyage or Cohere) first — same "call an API, get a score
back" shape as the LLM clients we already have, before reaching for a
self-hosted model or an LLM-based reranker.

## RAG evaluation: what's covered, and the real gap

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

## Other candidates

- **Auth hardening**: `/ingest` is gated by a single static bearer token
  (`ADMIN_TOKEN`); fine for a demo, not for multi-tenant or production use.
- **Context relevance/precision**: the retrieval eval measures recall (did we
  fetch the right products) but not precision (are the *other* candidates
  that got pulled in actively irrelevant noise) — a distinct RAG metric worth
  adding if retrieved-but-irrelevant products become a problem.
- **Richer input categorization**: `intent.Intent` collapses everything
  that isn't a product query into one `out_of_scope` bucket (greetings,
  spam, genuinely unanswerable-but-in-domain questions all get the same
  label/handling). Splitting these out would let "appropriate refusal rate"
  be measured separately from general answer quality, rather than folded
  into intent-classification accuracy.
