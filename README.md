# product-rag-search

Product info search with a RAG pipeline: intent classification, hybrid
retrieval (vector + graph + full-text), Reciprocal Rank Fusion re-ranking,
and grounded, cited answer generation.

## Stack

| Concern | Choice | Why |
|---|---|---|
| Language / API | Go, `net/http` (stdlib mux) | Single deployable binary, no framework needed at this size |
| Vector store | Postgres + pgvector | Cosine similarity search on the `products.embedding` column |
| Graph store | Postgres + Apache AGE | Cypher traversal on `product_graph`, same database as pgvector — one instance to operate, not two |
| Keyword search | Postgres full-text (`tsvector`/`ts_rank`) | Third retrieval signal, no extra service |
| Intent classification | Claude (`claude-haiku-4-5`) | Cheap/fast structured JSON output |
| Answer generation | Claude (`claude-sonnet-5`) | Citation-grounded generation via the Messages API |
| Embeddings | OpenAI (`text-embedding-3-small`) | Claude has no embeddings endpoint, so this is a required second provider |
| Re-ranking | Reciprocal Rank Fusion (pure Go) | No extra ML service; swappable behind an interface |
| Orchestration | Docker Compose | `postgres` → `migrate` → `app`, `seed` on demand |
| Migrations | `golang-migrate` | Plain SQL files, reviewable, no ORM |

## Architecture

```
                     ┌────────────────────┐
   POST /query  ───► │ intent.Classify     │  Claude Haiku, structured JSON:
                     │ (pkg/intent)        │  {intent, confidence, entities}
                     └─────────┬───────────┘
                               │
                 out_of_scope? │ yes ──► llm.Deflect (short reply, no retrieval)
                               │ no
                               ▼
                     ┌────────────────────┐
                     │ embeddings.Embed    │  OpenAI text-embedding-3-small
                     │ (pkg/embeddings)    │
                     └─────────┬───────────┘
                               │
              ┌────────────────┼─────────────────┐
              ▼                ▼                  ▼
        vector.Search    fulltext.Search     graph.Search        (run concurrently,
     (pgvector cosine)  (Postgres ts_rank)  (AGE Cypher, seeded    pkg/retrieval/*)
                                              from vector hits +
                                              classifier entities)
              └────────────────┼─────────────────┘
                               ▼
                     ┌────────────────────┐
                     │ rerank.Rerank       │  Reciprocal Rank Fusion:
                     │ (pkg/rerank/rrf)    │  score = Σ 1/(k + rank) per signal
                     └─────────┬───────────┘
                               ▼
                     ┌────────────────────┐
                     │ store.GetByIDs      │  hydrate top-N candidates into
                     │ (pkg/store)         │  full product records
                     └─────────┬───────────┘
                               ▼
                     ┌────────────────────┐
                     │ llm.GenerateAnswer  │  Claude Sonnet, products as `document`
                     │ (pkg/llm/claude)    │  blocks with citations enabled
                     └─────────┬───────────┘
                               ▼
                     {intent, answer, products[] with cited: bool}
```

**Ingestion flow** (`pkg/ingestion`, run via `cmd/seed` or `POST /ingest`):
for each raw product → embed (OpenAI) → insert into Postgres (`store.ProductRepository`)
→ upsert into the graph (`Product` vertex + `IN_CATEGORY`/`BY_BRAND`/`HAS_ATTRIBUTE`
edges) → wire up any curated `RELATED_TO` edges between products.

## Package layout

- `pkg/intent` — intent enum, entities, `Classifier` interface
- `pkg/embeddings` (+ `openai/`) — `Embedder` interface and its OpenAI implementation
- `pkg/llm` (+ `claude/`) — `ProductDoc`/`Answer` types, `Generator` interface, and the
  Claude implementation (also implements `intent.Classifier`)
- `pkg/retrieval` (+ `vector/`, `fulltext/`, `graph/`) — `Candidate`/`Query` types,
  `Source` interface, and its three implementations
- `pkg/rerank` (+ `rrf/`) — `Reranker` interface and the RRF implementation
- `pkg/store` (+ `postgres/`) — `Product` domain type, `ProductRepository` interface,
  and the Postgres implementation (pgxpool, AGE session setup)
- `pkg/ingestion` — embed → insert → graph-edge orchestration
- `pkg/pipeline` — the query flow above, wired from interfaces so every stage is swappable
- `pkg/router` — HTTP handlers (`/health`, `/query`, `/ingest`) and middleware
- `cmd/{server,migrate,seed}` — entrypoints

Every cross-cutting seam (embeddings, intent classification, generation, each retrieval
signal, re-ranking, product storage) is an interface with exactly one implementation
today. Swapping a provider or algorithm means adding a new implementation, not touching
callers — see [Future improvements](#future-improvements) for concrete candidates.

## Setup

```bash
cp .env.example .env
# fill in ANTHROPIC_API_KEY and OPENAI_API_KEY

docker compose build
docker compose up -d postgres migrate app
docker compose --profile seed run --rm seed   # loads seed/products.json
```

## Verify

```bash
curl http://localhost:8080/health

curl -s -XPOST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d '{"query":"comfortable running shoes under $100"}' | jq
```

Check the Postgres extensions and graph directly:

```bash
docker compose exec postgres psql -U postgres -d product_rag -c '\dx'
docker compose exec postgres psql -U postgres -d product_rag \
  -c "LOAD 'age'; SET search_path=ag_catalog,public; SELECT * FROM ag_graph;"
```

## Ingesting more products

`POST /ingest` (gated by `Authorization: Bearer $ADMIN_TOKEN`) accepts the
same shape as `seed/products.json`:

```json
{
  "products": [{ "title": "...", "description": "...", "category": "...", "brand": "...", "price": 0, "attributes": {} }],
  "relations": [{ "from_index": 0, "to_index": 1, "weight": 0.8 }]
}
```

## Tests

```bash
go test ./...
```

Unit tests use table-driven cases with `testify/assert`
(`pkg/rerank/rrf`, `pkg/retrieval/graph`). Integration tests behind a
`integration` build tag are not yet added — see below.

## Future improvements

### Alternative LLM / embeddings providers

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

### Other candidates

- **Integration tests** behind an `integration` build tag (`go test -tags=integration`),
  one `integration_test.go` per package that needs a live Postgres — not yet added.
- **Swap the reranker**: `rerank.Reranker` currently has one implementation (RRF).
  An LLM-based or cross-encoder reranker could be dropped in behind the same
  interface if RRF's relevance ceiling turns out to be too low in practice.
- **Auth hardening**: `/ingest` is gated by a single static bearer token
  (`ADMIN_TOKEN`); fine for a demo, not for multi-tenant or production use.
- **Observability**: structured request logging exists (`pkg/router/middleware.go`);
  no metrics/tracing yet.
