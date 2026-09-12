# Future improvements

- [Alternative LLM / embeddings providers](PROVIDERS.md): Gemini versus
  Claude's citations API, why DeepSeek was ruled out, and the swap path.
- [Alternative retrieval signals](RETRIEVAL.md): pgvector index tuning and
  dedicated vector databases, BM25 versus `ts_rank` and Elasticsearch/
  OpenSearch, and Apache AGE's missing indexes versus Neo4j/Memgraph or
  dropping the graph engine for plain Postgres joins.
- [Alternative rerankers](RERANKERS.md): RRF's ceiling, hosted
  cross-encoders (Voyage, Cohere), self-hosted, LLM-based.
- [RAG evaluation: what's covered, and the real gap](EVALUATION.md): the
  four implemented eval/monitoring areas, and why faithfulness and
  correctness of the generated answer still aren't measured (Ragas versus
  a hand-rolled judge).
- [Other candidates](OTHER.md): auth hardening, context precision, richer
  input categorization.
