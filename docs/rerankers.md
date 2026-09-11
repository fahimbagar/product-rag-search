# Alternative rerankers

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
