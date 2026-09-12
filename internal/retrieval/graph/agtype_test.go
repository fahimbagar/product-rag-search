package graph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVertex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		wantID  int64
		wantLbl string
		wantErr bool
	}{
		{
			// captured from a live psql session against apache/age:release_PG16_1.6.0
			name:    "real vertex fixture",
			raw:     `{"id": 844424930131969, "label": "Product", "properties": {"title": "Test Shoe", "product_id": "abc-123"}}::vertex`,
			wantID:  844424930131969,
			wantLbl: "Product",
		},
		{
			name:    "vertex with empty properties",
			raw:     `{"id": 1, "label": "Category", "properties": {}}::vertex`,
			wantID:  1,
			wantLbl: "Category",
		},
		{
			name:    "missing vertex suffix is an error",
			raw:     `{"id": 1, "label": "Category", "properties": {}}`,
			wantErr: true,
		},
		{
			name:    "malformed json is an error",
			raw:     `{not json}::vertex`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v, err := ParseVertex(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantID, v.ID)
			assert.Equal(t, tt.wantLbl, v.Label)
		})
	}
}

func TestParseString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{name: "real scalar string fixture", raw: `"abc-123"`, want: "abc-123"},
		{name: "empty string", raw: `""`, want: ""},
		{name: "unquoted bareword is an error", raw: `abc-123`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseString(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseFloat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    float64
		wantErr bool
	}{
		{name: "real count(n) fixture", raw: "1", want: 1},
		{name: "larger count with surrounding whitespace", raw: "  42 ", want: 42},
		{name: "real sum(r.weight) fixture", raw: "0.8", want: 0.8},
		{name: "quoted string is an error", raw: `"1"`, wantErr: true},
		{name: "empty string is an error", raw: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseFloat(tt.raw)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
