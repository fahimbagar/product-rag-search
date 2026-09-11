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

## Other candidates

- **Integration tests** behind an `integration` build tag (`go test -tags=integration`),
  one `integration_test.go` per package that needs a live Postgres — not yet added.
- **Swap the reranker**: `rerank.Reranker` currently has one implementation (RRF).
  An LLM-based or cross-encoder reranker could be dropped in behind the same
  interface if RRF's relevance ceiling turns out to be too low in practice.
- **Auth hardening**: `/ingest` is gated by a single static bearer token
  (`ADMIN_TOKEN`); fine for a demo, not for multi-tenant or production use.
- **Observability**: structured request logging exists (`pkg/router/middleware.go`);
  no metrics/tracing yet.
