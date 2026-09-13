# Alternative retrieval signals

The pipeline fans out three `retrieval.Source` implementations concurrently
(`internal/retrieval/vector`, `fulltext`, `graph`) and merges their rankings
with Reciprocal Rank Fusion (`internal/rerank/rrf`). RRF only reads each
candidate's rank within a signal, not its raw score, so swapping any one
signal for an alternative changes ordering inside that signal without
touching the fusion step or the other two sources. Each section below covers
the current implementation, its ceiling, and what could replace it.

## Vector search

Current: pgvector's `<=>` cosine-distance operator over the `products.embedding`
column, backed by an HNSW index (`vector_cosine_ops`, default build
parameters, no `m`/`ef_construction` tuning), searching 1536-dim
`gemini-embedding-001` vectors.

**Index tuning before swapping anything.** HNSW's defaults trade off recall
against build time and memory; raising `m`/`ef_construction` at index-build
time or `hnsw.ef_search` per query buys recall at the cost of latency.
pgvector also supports an IVFFlat index, which builds faster and uses less
memory but needs `lists` tuned to the row count and an `ANALYZE` pass to
train the clusters, and it typically loses to HNSW on recall at query time.
Given the current 24-product seed catalog, neither index type's tuning
matters yet; it becomes relevant once the catalog is large enough to notice
differences in recall or latency.

**Dedicated vector databases** (Pinecone, Weaviate, Qdrant, Milvus) scale
past what a single Postgres instance's HNSW index handles well and add
native metadata filtering, but each is a separate service to run and
operate. The same "one Postgres instance for vector and graph" rationale
that keeps Apache AGE alongside pgvector (see the [Stack](../README.md#stack))
argues against this until pgvector's own ceiling becomes the actual bottleneck.

**Swapping the embedding model** is a smaller, more likely change:
`embeddings.Embedder` is already an interface (see
[Alternative LLM / embeddings providers](PROVIDERS.md)), so trying OpenAI's
`text-embedding-3`, Cohere's `embed`, or Voyage's embedding models means a
new implementation and a new `vector(N)` column dimension, not a pipeline
rewrite. The tradeoff is losing the current setup's single-provider,
permanent-free-tier property.

## Full-text search

Current: a generated `tsvector` column (`title` weighted `A`, `description`
weighted `B`, `category`/`brand` weighted `C`), queried with
`websearch_to_tsquery('english', ...)` and ranked by `ts_rank`, backed by a
GIN index.

**`ts_rank` is not BM25.** It weights term frequency by the configured
letter weights but does not do BM25's document-length normalization or term
saturation, so a long description repeating a keyword can outrank a short,
more relevant one. `pg_search` (the ParadeDB extension, built on Tantivy)
adds real BM25 scoring inside Postgres, staying in the same database and
process model this project already favors, at the cost of one more
extension to install and maintain alongside `pgvector` and `age`.

**Elasticsearch or OpenSearch** give BM25 ranking, per-field boosting,
fuzzy matching, and language analyzers out of the box, and are the
established choice for hybrid search at scale (the same systems RRF itself
comes from). Both mean a separate service, a second data store to keep in
sync with Postgres, and cluster operations this project's single-instance
philosophy has avoided everywhere else.

**Typo tolerance without leaving Postgres**: `pg_trgm` (trigram similarity)
layered on top of the existing `tsvector` column handles misspelled product
names and brand names that `websearch_to_tsquery`'s exact-token matching
currently misses, without adding a service or a rank normalization change.

**Multi-language catalogs**: the `'english'` config is hardcoded in both the
generated column and the query. A catalog with non-English titles or
descriptions needs either a `regconfig` column driven by product locale or
a separate `tsvector` per language, since `to_tsvector` picks stemming rules
per call.

## Graph traversal

Current: Postgres + Apache AGE, Cypher queries built with `fmt.Sprintf`
string interpolation (AGE has no official Go client and its own parameter
binding is unreliable across versions, per the package doc), guarded by a
UUID regex, a label allowlist regex, and manual escaping. `IN_CATEGORY`,
`BY_BRAND`, and `HAS_ATTRIBUTE` edges score by hop count; `RELATED_TO`
edges score by curated `weight` on direct matches only (see the
["Graph proximity scoring"](../README.md#architecture) section for why weight doesn't extend across
multi-hop paths).

**No index exists on the traversed properties.** `0002_age_graph.up.sql`
only loads the extension and creates the graph; there's no index on
`Product.product_id`, `Category.name`, or `Brand.name`, so every `MATCH` in
`store.go` is an unindexed scan over each label's underlying table. AGE
stores each label as a regular Postgres table with the properties in a
`jsonb` column, so a standard expression index fixes this without changing
the traversal logic or swapping engines:
`CREATE INDEX ON product_graph."Product" USING btree ((properties->>'product_id'))`
(and the equivalent for `Category.name`/`Brand.name`). This is the cheapest
fix and should happen before considering anything below.

**Neo4j or Memgraph** replace `fmt.Sprintf`-built Cypher with a real driver
(`neo4j-go-driver`, parameterized queries, no string-interpolation
surface at all) and native property indexes instead of AGE's jsonb scans.
Memgraph is Bolt-protocol compatible with the same driver and runs
in-memory for lower query latency. Both are a separate service from
Postgres, which is the exact tradeoff AGE was chosen to avoid (see the
Stack table's "one instance to operate, not two").

**Dropping the graph engine entirely**: category/brand/attribute
relationships and curated product-to-product links could be modeled as
plain Postgres tables (`category_id`/`brand_id` foreign keys, a
`product_attributes` join table, a `product_relations` table with a
`weight` column) and traversed with recursive CTEs instead of Cypher. Given
the current traversal depth is capped at two hops, a CTE join likely
performs comparably to AGE while getting real B-tree indexes on foreign
keys and removing the injection-adjacent string-building path entirely.
The tradeoff is losing Cypher's pattern-matching expressiveness if the
traversal patterns ever grow past what a two-hop join comfortably
expresses.

`retrieval.Source` is the interface each of the above would implement, so
none of these changes touch `internal/pipeline` or `internal/rerank`.
