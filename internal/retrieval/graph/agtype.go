package graph

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Vertex is the decoded form of an agtype vertex literal, e.g.
// `{"id": 844424930131969, "label": "Product", "properties": {...}}::vertex`.
// Apache AGE has no official Go client, so this parsing is hand-rolled
// against fixtures captured from a live psql session (see docker/postgres
// validation notes).
type Vertex struct {
	ID         int64
	Label      string
	Properties map[string]any
}

func ParseVertex(raw string) (Vertex, error) {
	body, ok := strings.CutSuffix(strings.TrimSpace(raw), "::vertex")
	if !ok {
		return Vertex{}, fmt.Errorf("agtype: not a vertex literal: %s", raw)
	}
	var v struct {
		ID         int64          `json:"id"`
		Label      string         `json:"label"`
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		return Vertex{}, fmt.Errorf("agtype: parse vertex: %w", err)
	}
	return Vertex{ID: v.ID, Label: v.Label, Properties: v.Properties}, nil
}

// ParseString parses a scalar agtype projection that is a JSON string, e.g.
// `"abc-123"` (no ::type suffix; only vertex/edge/path literals carry one).
func ParseString(raw string) (string, error) {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return "", fmt.Errorf("agtype: parse string %q: %w", raw, err)
	}
	return s, nil
}

// ParseFloat parses a scalar agtype projection that is a bare number, e.g.
// `1` from count(*) or `0.8` from sum(r.weight).
func ParseFloat(raw string) (float64, error) {
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("agtype: parse float %q: %w", raw, err)
	}
	return n, nil
}
