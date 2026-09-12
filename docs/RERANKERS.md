# Alternative rerankers

The current reranker is Reciprocal Rank Fusion (`internal/rerank/rrf`), a
pure-Go formula (`score = Σ 1/(k + rank)`) that merges the three retrieval
signals' rank *positions*, with no understanding of the query or document
text itself. It was chosen because it's free, needs no extra infrastructure,
and is a well-established technique (used in production by
Elasticsearch/OpenSearch hybrid search). It has a real ceiling: two
candidates ranked #1 by different signals for different reasons score the
same as two candidates that are both excellent matches. RRF cannot tell a
coincidental rank agreement apart from genuine relevance.

`rerank.Reranker` is the interface (`internal/rerank/rerank.go`), so a
better-quality reranker can be swapped in later without touching the
pipeline. The candidates, in increasing order of quality and cost:

**Hosted cross-encoder rerank APIs.** A cross-encoder processes the query
and each candidate document's text (title, description, attributes)
together, not just its rank position, so it can judge relevance instead of
agreement between signals. Two managed options fit our `Reranker` interface
as a drop-in (query and documents in, reranked scores out):

- **Voyage AI rerank** (`rerank-2.5` / `rerank-3`): a different vendor than
  our current Gemini embeddings, but the same "call an API, get scores
  back" shape.
- **Cohere Rerank** (`rerank-v4.0-pro` / `-fast`): same shape, different
  vendor.

Both require an extra API key and a network call per query (added latency
and cost), but no extra infrastructure to run ourselves.

**Self-hosted cross-encoder** (for example, `bge-reranker-v2-m3`) via a
small Python/ONNX sidecar container. This avoids per-query API cost and an
external network hop, but adds a service to build, deploy, and keep
patched. It is the one option that conflicts with the "simple for
production, easy to maintain" requirement that led to choosing RRF in the
first place. It's worth the added service only if RRF's relevance ceiling
turns out to be a real problem and the hosted APIs' cost or latency at
volume rules them out too.

**LLM-based reranker.** Feed the top-K candidates back to Claude or Gemini
and ask it to score or reorder them. This has the highest potential quality
(full reasoning over text, can weigh business rules), but the highest
latency and cost per query of any option, and is the least deterministic.

**Recommendation if RRF turns out to be the bottleneck:** try a hosted
cross-encoder (Voyage or Cohere) first. It has the same "call an API, get a
score back" shape as the LLM clients already in this repo, before reaching
for a self-hosted model or an LLM-based reranker.
