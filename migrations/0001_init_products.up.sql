CREATE EXTENSION IF NOT EXISTS vector;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE products (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT NOT NULL,
    description   TEXT NOT NULL,
    category      TEXT NOT NULL,
    brand         TEXT NOT NULL,
    price         NUMERIC(12, 2) NOT NULL,
    attributes    JSONB NOT NULL DEFAULT '{}',
    embedding     vector(1536),
    search_tsv    tsvector GENERATED ALWAYS AS (
                      setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
                      setweight(to_tsvector('english', coalesce(description, '')), 'B') ||
                      setweight(to_tsvector('english', coalesce(category, '')), 'C') ||
                      setweight(to_tsvector('english', coalesce(brand, '')), 'C')
                  ) STORED,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX products_search_tsv_idx ON products USING GIN (search_tsv);
CREATE INDEX products_embedding_hnsw_idx ON products USING hnsw (embedding vector_cosine_ops);
