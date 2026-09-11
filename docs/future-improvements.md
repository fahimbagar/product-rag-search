# Future improvements

## Alternative LLM / embeddings providers

Claude + OpenAI is the current default, but both seams are interfaces
(`intent.Classifier`/`llm.Generator` and `embeddings.Embedder`), so either can be
swapped independently.

**Embeddings (low-risk swap):** OpenAI → **Voyage AI** (Anthropic's recommended
embeddings partner) or **Google Gemini embeddings** are drop-in alternatives — "text
in, vector out," no feature-parity concerns. Gemini's is notable because it's on a
genuinely **permanent free tier** (verified against Google's pricing docs), unlike
OpenAI/Anthropic's one-time trial credits.

**LLM / generation (real tradeoff):** the main reason to stay on Claude is its
`citations` API. We send each retrieved product as a `document` content block with
`citations: {enabled: true}`, and Claude's response comes back with each answer
segment linked to the exact source document and the exact quoted span of text that
supports it — the API computes and verifies this; the model cannot fake a citation.
That's what backs the `cited: bool` flag on each product in the `/query` response.

Switching generation to **Google Gemini** (or DeepSeek, Mistral, etc.) would mean
losing that verification. None of them have an equivalent document-citation
mechanism. The workaround is structured output: define a JSON schema like
`{"answer": "string", "cited_product_ids": ["string"]}` and instruct the model to
self-report which products it used. The schema guarantees the *shape* of the
response, but nothing checks whether the reported IDs are actually what the answer
text relied on — it's the model's word for it, not an API-verified fact. Practically,
the `cited` flag would go from "verified" to "the model's best guess."

(DeepSeek was also considered and ruled out for now: it has no embeddings API at
all, so it can only ever replace the LLM half, not the embeddings half — and it
has the same citation-verification gap as Gemini.)

**Open decision:** switch both roles to Gemini's free tier (accepting the citation
downgrade), switch embeddings only, or stay on Claude + OpenAI. Revisit once cost
at real usage volume is known.

## Alternative rerankers

The current reranker is Reciprocal Rank Fusion (`pkg/rerank/rrf`) — a pure-Go
formula (`score = Σ 1/(k + rank)`) that merges the three retrieval signals'
rank *positions*, with no understanding of the query or document text itself.
It was chosen because it's free, has no extra infra, and is a well-established
technique (used in production by Elasticsearch/OpenSearch hybrid search), but
it has a real ceiling: two candidates ranked #1 by different signals for
completely different reasons score identically to two candidates that are
genuinely both excellent matches — RRF can't tell "coincidentally ranked
similarly" apart from "actually relevant."

`rerank.Reranker` is the interface (`pkg/rerank/rerank.go`) precisely so a
better-quality reranker can be swapped in later without touching the
pipeline. The candidates, in increasing order of quality/cost:

**Hosted cross-encoder rerank APIs** — a cross-encoder jointly processes the
query and each candidate document's *text* (title/description/attributes),
not just its rank position, so it can actually judge relevance rather than
agreement between signals. Two managed options that fit our `Reranker`
interface as a drop-in (query + documents in, reranked scores out):
- **Voyage AI rerank** (`rerank-2.5` / `rerank-3`) — pairs naturally with
  Voyage embeddings if we ever switch off OpenAI (see above).
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

## Other candidates

- **Integration tests** behind an `integration` build tag (`go test -tags=integration`),
  one `integration_test.go` per package that needs a live Postgres — not yet added.
- **Auth hardening**: `/ingest` is gated by a single static bearer token
  (`ADMIN_TOKEN`); fine for a demo, not for multi-tenant or production use.
- **Observability**: structured request logging exists (`pkg/router/middleware.go`);
  no metrics/tracing yet.
