package router

import "github.com/fahimbagar/product-rag-search/internal/pipeline"

type queryRequest struct {
	Query string `json:"query"`
}

type productResponse struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Brand    string  `json:"brand"`
	Category string  `json:"category"`
	Price    float64 `json:"price"`
	Cited    bool    `json:"cited"`
}

type queryResponse struct {
	Intent   string            `json:"intent"`
	Answer   string            `json:"answer"`
	Products []productResponse `json:"products"`
}

func toQueryResponse(r pipeline.Result) queryResponse {
	products := make([]productResponse, len(r.Products))
	for i, p := range r.Products {
		products[i] = productResponse{
			ID:       p.ID,
			Title:    p.Title,
			Brand:    p.Brand,
			Category: p.Category,
			Price:    p.Price,
			Cited:    p.Cited,
		}
	}
	return queryResponse{
		Intent:   string(r.Intent),
		Answer:   r.Answer,
		Products: products,
	}
}
