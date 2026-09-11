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
| Intent classification | Gemini (`gemini-3.5-flash-lite`) | Cheap/fast structured JSON output, permanent free tier |
| Answer generation | Gemini (`gemini-3.8-flash`) | Structured-output generation with self-reported product citations |
| Embeddings | Gemini (`gemini-embedding-001`, 1536-dim) | Same provider/API key as generation, permanent free tier |
| Re-ranking | Reciprocal Rank Fusion (pure Go) | No extra ML service; swappable behind an interface |
| Orchestration | Docker Compose | `postgres` → `migrate` → `app`, `seed` on demand |
| Migrations | `golang-migrate` | Plain SQL files, reviewable, no ORM |

## Architecture

```
                     ┌────────────────────┐
   POST /query  ───► │ intent.Classify     │  Gemini Flash-Lite, structured JSON:
                     │ (pkg/intent)        │  {intent, confidence, entities}
                     └─────────┬───────────┘
                               │
                 out_of_scope? │ yes ──► llm.Deflect (short reply, no retrieval)
                               │ no
                               ▼
                     ┌────────────────────┐
                     │ embeddings.Embed    │  Gemini gemini-embedding-001
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
                     │ llm.GenerateAnswer  │  Gemini Flash, structured JSON:
                     │ (pkg/llm/gemini)    │  {answer, cited_product_ids} (self-reported)
                     └─────────┬───────────┘
                               ▼
                     {intent, answer, products[] with cited: bool}
```

**Ingestion flow** (`pkg/ingestion`, run via `cmd/seed` or `POST /ingest`):
for each raw product → embed (Gemini) → insert into Postgres (`store.ProductRepository`)
→ upsert into the graph (`Product` vertex + `IN_CATEGORY`/`BY_BRAND`/`HAS_ATTRIBUTE`
edges) → wire up any curated `RELATED_TO` edges between products.

## Package layout

- `pkg/intent` — intent enum, entities, `Classifier` interface
- `pkg/embeddings` (+ `gemini/`) — `Embedder` interface and its Gemini implementation
- `pkg/llm` (+ `gemini/`) — `ProductDoc`/`Answer` types, `Generator` interface, and the
  Gemini implementation (also implements `intent.Classifier`)
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
# fill in GEMINI_API_KEY

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
`integration` build tag are not yet added — see
[docs/future-improvements.md](docs/future-improvements.md).

## Future improvements

See [docs/future-improvements.md](docs/future-improvements.md) — alternative
LLM/embeddings providers (and the citation-verification tradeoff involved),
integration tests, reranker swaps, auth hardening, and observability.
