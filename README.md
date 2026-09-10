# product-rag-search

Product info search with a RAG pipeline: intent classification, hybrid
retrieval (vector + graph + full-text), Reciprocal Rank Fusion re-ranking,
and grounded, cited answer generation.

## Architecture

- **Postgres + pgvector + Apache AGE** — one database for vector similarity
  search, graph traversal (Cypher via AGE), and full-text search, all on the
  `products` table / `product_graph` graph.
- **Intent classification** — Claude (`claude-haiku-4-5` by default) labels
  each query as `product_search`, `product_comparison`,
  `product_attribute_question`, or `out_of_scope`, and extracts
  brand/category/attribute entities used to seed graph traversal directly.
- **Retrieval** — three signals run concurrently: pgvector cosine
  similarity, Apache AGE graph-proximity traversal (category/brand/attribute/
  related-product edges), and Postgres full-text search (`tsvector`/
  `ts_rank`).
- **Re-ranking** — Reciprocal Rank Fusion (pure Go, no extra service) merges
  the three ranked lists behind a `Reranker` interface, so an LLM or
  cross-encoder reranker can be swapped in later without touching callers.
- **Generation** — Claude (`claude-sonnet-5` by default) generates an answer
  grounded in the retrieved products via the Messages API's `citations`
  feature, so the response can report exactly which products it relied on.

See `pkg/` for the package layout: `intent`, `embeddings`, `llm`,
`retrieval/{vector,graph,fulltext}`, `rerank/rrf`, `ingestion`, `pipeline`,
`store`, `router`.

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
`integration` build tag are not yet added.
