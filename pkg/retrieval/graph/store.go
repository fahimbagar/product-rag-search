// Package graph implements retrieval.Source (proximity search) and a
// GraphWriter (ingestion writes) against the Apache AGE "product_graph".
//
// AGE's cypher() parameter binding is unreliable across versions, so query
// bodies are built via string interpolation. Every interpolated value is
// either matched against a strict allowlist pattern or escaped as a Cypher
// string literal before it goes anywhere near a query.
package graph

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fahimbagar/product-rag-search/pkg/retrieval"
)

var (
	uuidPattern  = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	labelPattern = regexp.MustCompile(`^[a-zA-Z0-9_ &'-]{1,128}$`)
)

// Writer is the graph-ingestion seam: turning a product record into graph
// nodes/edges. Implemented by Store; consumed by pkg/ingestion.
type Writer interface {
	UpsertProduct(ctx context.Context, p ProductNode) error
	LinkRelated(ctx context.Context, fromProductID, toProductID string, weight float64) error
}

type ProductNode struct {
	ProductID  string
	Title      string
	Category   string
	Brand      string
	Attributes map[string]string
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Name() string { return "graph" }

// Search implements retrieval.Source: proximity is the number of
// category/brand/attribute/related-product hops connecting a candidate to
// the query's seed products, category, or brand.
func (s *Store) Search(ctx context.Context, q retrieval.Query) ([]retrieval.Candidate, error) {
	proximity := map[string]int64{}

	var seedIDs []string
	for _, id := range q.SeedIDs {
		if uuidPattern.MatchString(id) {
			seedIDs = append(seedIDs, id)
		}
	}
	if len(seedIDs) > 0 {
		if err := s.accumulateBySeedIDs(ctx, seedIDs, proximity); err != nil {
			return nil, err
		}
	}

	if q.Category != "" && labelPattern.MatchString(q.Category) {
		if err := s.accumulateByLabel(ctx, "Category", q.Category, proximity); err != nil {
			return nil, err
		}
	}

	if q.Brand != "" && labelPattern.MatchString(q.Brand) {
		if err := s.accumulateByLabel(ctx, "Brand", q.Brand, proximity); err != nil {
			return nil, err
		}
	}

	return rankCandidates(proximity, s.Name(), q.TopK), nil
}

func (s *Store) accumulateBySeedIDs(ctx context.Context, seedIDs []string, proximity map[string]int64) error {
	quoted := make([]string, len(seedIDs))
	for i, id := range seedIDs {
		quoted[i] = "'" + id + "'"
	}

	query := fmt.Sprintf(`
		SELECT * FROM cypher('product_graph', $$
			MATCH (seed:Product)-[:IN_CATEGORY|BY_BRAND|HAS_ATTRIBUTE|RELATED_TO*1..2]-(related:Product)
			WHERE seed.product_id IN [%s] AND related.product_id <> seed.product_id
			RETURN related.product_id, count(*)
		$$) AS (product_id agtype, proximity agtype)
	`, strings.Join(quoted, ", "))

	return s.runProximityQuery(ctx, query, proximity)
}

func (s *Store) accumulateByLabel(ctx context.Context, label, name string, proximity map[string]int64) error {
	query := fmt.Sprintf(`
		SELECT * FROM cypher('product_graph', $$
			MATCH (n:%s {name: '%s'})<-[:IN_CATEGORY|BY_BRAND]-(related:Product)
			RETURN related.product_id, count(*)
		$$) AS (product_id agtype, proximity agtype)
	`, label, cypherEscape(name))

	return s.runProximityQuery(ctx, query, proximity)
}

func (s *Store) runProximityQuery(ctx context.Context, query string, proximity map[string]int64) error {
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("graph search: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rawID, rawProximity string
		if err := rows.Scan(&rawID, &rawProximity); err != nil {
			return fmt.Errorf("graph search: scan: %w", err)
		}
		id, err := ParseString(rawID)
		if err != nil {
			return err
		}
		count, err := ParseInt(rawProximity)
		if err != nil {
			return err
		}
		proximity[id] += count
	}
	return rows.Err()
}

func rankCandidates(proximity map[string]int64, source string, topK int) []retrieval.Candidate {
	if len(proximity) == 0 {
		return nil
	}

	ids := make([]string, 0, len(proximity))
	for id := range proximity {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if proximity[ids[i]] != proximity[ids[j]] {
			return proximity[ids[i]] > proximity[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if topK > 0 && len(ids) > topK {
		ids = ids[:topK]
	}

	candidates := make([]retrieval.Candidate, len(ids))
	for i, id := range ids {
		candidates[i] = retrieval.Candidate{
			ProductID: id,
			Rank:      i + 1,
			Score:     float64(proximity[id]),
			Source:    source,
		}
	}
	return candidates
}

// UpsertProduct creates (or updates) the Product vertex and its
// category/brand/attribute edges. Idempotent via MERGE.
func (s *Store) UpsertProduct(ctx context.Context, p ProductNode) error {
	var b strings.Builder
	fmt.Fprintf(&b, `
		SELECT * FROM cypher('product_graph', $$
			MERGE (p:Product {product_id: '%s'})
			SET p.title = '%s'
	`, cypherEscape(p.ProductID), cypherEscape(p.Title))

	if p.Category != "" {
		fmt.Fprintf(&b, `
			MERGE (c:Category {name: '%s'})
			MERGE (p)-[:IN_CATEGORY]->(c)
		`, cypherEscape(p.Category))
	}
	if p.Brand != "" {
		fmt.Fprintf(&b, `
			MERGE (br:Brand {name: '%s'})
			MERGE (p)-[:BY_BRAND]->(br)
		`, cypherEscape(p.Brand))
	}

	b.WriteString(`
			RETURN p
		$$) AS (p agtype)
	`)

	if _, err := s.pool.Exec(ctx, b.String()); err != nil {
		return fmt.Errorf("graph upsert product: %w", err)
	}

	keys := make([]string, 0, len(p.Attributes))
	for k := range p.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := s.linkAttribute(ctx, p.ProductID, key, p.Attributes[key]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) linkAttribute(ctx context.Context, productID, key, value string) error {
	query := fmt.Sprintf(`
		SELECT * FROM cypher('product_graph', $$
			MATCH (p:Product {product_id: '%s'})
			MERGE (a:Attribute {key: '%s', value: '%s'})
			MERGE (p)-[:HAS_ATTRIBUTE]->(a)
			RETURN a
		$$) AS (a agtype)
	`, cypherEscape(productID), cypherEscape(key), cypherEscape(value))

	if _, err := s.pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("graph link attribute: %w", err)
	}
	return nil
}

// LinkRelated creates an explicit curated RELATED_TO edge between two
// products (e.g. a matching accessory), distinct from the implicit
// category/brand/attribute proximity.
func (s *Store) LinkRelated(ctx context.Context, fromProductID, toProductID string, weight float64) error {
	query := fmt.Sprintf(`
		SELECT * FROM cypher('product_graph', $$
			MATCH (a:Product {product_id: '%s'}), (b:Product {product_id: '%s'})
			MERGE (a)-[r:RELATED_TO]->(b)
			SET r.weight = %f
			RETURN r
		$$) AS (r agtype)
	`, cypherEscape(fromProductID), cypherEscape(toProductID), weight)

	if _, err := s.pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("graph link related: %w", err)
	}
	return nil
}

func cypherEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return s
}
