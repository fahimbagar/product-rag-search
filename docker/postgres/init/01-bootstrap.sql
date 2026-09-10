-- The base apache/age image already runs 00-create-extension-age.sql
-- (plain `CREATE EXTENSION age;`, no IF NOT EXISTS), so this file only adds
-- pgvector and creates the app graph, and is named to sort after it.
CREATE EXTENSION IF NOT EXISTS vector;

LOAD 'age';
SET search_path = ag_catalog, "$user", public;

SELECT create_graph('product_graph');
