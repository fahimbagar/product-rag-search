# product-rag-search

[![static analysis](https://github.com/fahimbagar/product-rag-search/actions/workflows/static-analysis.yml/badge.svg)](https://github.com/fahimbagar/product-rag-search/actions/workflows/static-analysis.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Product info search with a RAG pipeline: intent classification, hybrid
retrieval (vector + graph + full-text), Reciprocal Rank Fusion re-ranking,
and grounded, cited answer generation.

> **This is a showcase/demo project, not a production build.** It
> demonstrates the architecture (hybrid retrieval, RRF fusion, structured
> LLM generation, metrics/eval) end to end, but skips things a real
> deployment would need: `/ingest` auth is a single static bearer token,
> there's no rate limiting or multi-tenant isolation, the seed catalog is
> 24 fake products, and generated-answer faithfulness/correctness isn't
> verified (see [docs/FUTURE-IMPROVEMENTS.md](docs/FUTURE-IMPROVEMENTS.md)).

## Stack

| Concern | Choice | Why |
|---|---|---|
| Language / API | Go, `go-chi` | Single deployable binary, thin routing on top of `net/http` |
| Vector store | Postgres + pgvector | Cosine similarity search on the `products.embedding` column |
| Graph store | Postgres + Apache AGE | Cypher traversal on `product_graph`, same database as pgvector (one instance to operate, not two) |
| Keyword search | Postgres full-text (`tsvector`/`ts_rank`) | Third retrieval signal, no extra service |
| Intent classification | Gemini (`gemini-3.5-flash-lite`) | Cheap/fast structured JSON output, permanent free tier |
| Answer generation | Gemini (`gemini-3.8-flash`) | Structured-output generation with self-reported product citations |
| Embeddings | Gemini (`gemini-embedding-001`, 1536-dim) | Same provider/API key as generation, permanent free tier |
| Re-ranking | Reciprocal Rank Fusion (pure Go) | No extra ML service; swappable behind an interface |
| Orchestration | Docker Compose | `postgres` → `migrate` → `app`, `seed` on demand |
| Migrations | `golang-migrate` | Plain SQL files, reviewable, no ORM |
| Metrics | Prometheus (`client_golang`) + Grafana | Go-native metrics client, `GET /metrics`, pre-provisioned dashboard |
| Logging | `zap`, JSON to stdout | Structured, request-scoped logger carried on `context.Context` (`internal/ctxlog`), not threaded through every function signature |

## Architecture

```
POST /query
  │
  ▼
1. intent.Classify (internal/intent)                Gemini Flash-Lite, structured JSON:
                                                      {intent, confidence, entities}
  │
  ├─ out_of_scope? ──yes──► llm.Deflect (short reply, no retrieval, return)
  │  no
  ▼
2. embeddings.Embed (internal/embeddings)           Gemini gemini-embedding-001
  │
  ▼
3. fan out concurrently (internal/retrieval/*):
     vector.Search    : pgvector cosine similarity
     fulltext.Search  : Postgres ts_rank
     graph.Search     : AGE Cypher, seeded from vector hits + classifier entities
  │
  ▼
4. rerank.Rerank (internal/rerank/rrf)               Reciprocal Rank Fusion:
                                                      score = Σ 1/(k + rank) per signal
  │
  ▼
5. store.GetByIDs (internal/store)                   hydrate top-N candidates into
                                                      full product records
  │
  ▼
6. llm.GenerateAnswer (internal/llm/gemini)           Gemini Flash, structured JSON:
                                                      {answer, cited_product_ids} (self-reported)
  │
  ▼
{intent, answer, products[] with cited: bool}
```

**Ingestion flow** (`internal/ingestion`, run via `cmd/seed` or `POST /ingest`):
for each raw product → embed (Gemini) → insert into Postgres (`store.ProductRepository`)
→ upsert into the graph (`Product` vertex + `IN_CATEGORY`/`BY_BRAND`/`HAS_ATTRIBUTE`
edges) → wire up any curated `RELATED_TO` edges between products.

## Package layout

- `internal/intent`: intent enum, entities, `Classifier` interface
- `internal/embeddings` (+ `gemini/`): `Embedder` interface and its Gemini implementation
- `internal/llm` (+ `gemini/`): `ProductDoc`/`Answer` types, `Generator` interface, and the
  Gemini implementation (also implements `intent.Classifier`)
- `internal/retrieval` (+ `vector/`, `fulltext/`, `graph/`): `Candidate`/`Query` types,
  `Source` interface, and its three implementations
- `internal/rerank` (+ `rrf/`): `Reranker` interface and the RRF implementation
- `internal/store` (+ `postgres/`): `Product` domain type, `ProductRepository` interface,
  and the Postgres implementation (pgxpool, AGE session setup)
- `internal/ingestion`: embed, insert, then wire graph edges
- `internal/pipeline`: the query flow above, wired from interfaces so every stage is swappable
- `internal/router`: HTTP handlers (`/query`, `/ingest`) and middleware, built on `go-chi`
- `internal/app`: loads config and wires every dependency above into concrete types,
  shared by `cmd/server` and `cmd/seed`
- `internal/healthcheck`: `/health` handler backed by a `Pinger` (DB pool), decoupled from `internal/router`
- `internal/ctxlog`: carries the request-scoped `*zap.SugaredLogger` on `context.Context`
- `cmd/{server,migrate,seed}`: entrypoints

Every cross-cutting seam (embeddings, intent classification, generation, each retrieval
signal, re-ranking, product storage) is an interface with exactly one implementation
today. Swapping a provider or algorithm means adding a new implementation, not touching
callers. See [Future improvements](#future-improvements) for concrete candidates.

## Setup

```bash
cp .env.example .env
# fill in GEMINI_API_KEY

docker compose build
docker compose up -d postgres migrate app prometheus grafana
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

## Monitoring & evaluation

**Runtime metrics** (`GET /metrics`, Prometheus format, unauthenticated like `/health`):
HTTP request rate/latency/errors, per-pipeline-stage latency (classify, embed,
each retrieval source, rerank, hydrate, generate, deflect), which retrieval
signal(s) contributed each final result, self-reported citation valid vs.
hallucinated counts, Gemini token usage, and intent classification counts.

- Prometheus: http://localhost:9090 (scrapes `app:8080/metrics` every 15s)
- Grafana: http://localhost:3000 (anonymous viewer access; auto-provisioned
  "Product RAG Search" dashboard, datasource pre-configured, no manual setup)

**Offline evals** (`integration_tests/`, behind the `integration` build
tag, need a live seeded Postgres + a real `GEMINI_API_KEY`):

```bash
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/product_rag?sslmode=disable'
export GEMINI_API_KEY='...'
go test -tags=integration ./integration_tests/... -v
```

- `retrieval_eval_test.go`: maps a golden query to expected product titles
  (matched by title, not ID, since seed IDs are regenerated every run),
  scored as recall@FinalTopN against the real vector+fulltext+graph+RRF
  pipeline.
- `intent_eval_test.go`: maps a golden query to an expected intent, scored
  as accuracy against the real Gemini classifier.

Both tests fail the run when the aggregate score drops below a threshold
(`minRetrievalRecall`, `minIntentAccuracy`), so a retrieval or classifier
regression blocks CI.

See [docs/FUTURE-IMPROVEMENTS.md](docs/FUTURE-IMPROVEMENTS.md) for what these
five areas do *not* cover (faithfulness/correctness of the generated answer
text) and why.

## Tests

```bash
go test ./...
```

Unit tests use table-driven cases with `testify/assert`
(`internal/rerank/rrf`, `internal/retrieval/graph`).

## Future improvements

See [docs/FUTURE-IMPROVEMENTS.md](docs/FUTURE-IMPROVEMENTS.md) for alternative
LLM/embeddings providers (and the citation-verification tradeoff involved),
integration tests, reranker swaps, auth hardening, and observability.

## License

[MIT](LICENSE)
