# Alternative LLM / embeddings providers

Gemini (`gemini-3.5-flash-lite` for intent, `gemini-3.8-flash` for generation,
`gemini-embedding-001` for embeddings) is the current default: one provider,
one API key, and a **permanent free tier** for both chat and embeddings
(verified against Google's pricing docs), unlike OpenAI's or Anthropic's
one-time trial credits. Both seams are interfaces (`intent.Classifier`/
`llm.Generator` and `embeddings.Embedder`), so either can be swapped
independently if cost stops being the deciding factor.

**Claude: the prior choice here, and the upgrade path for verified
citations.** This project originally used Claude (`claude-haiku-4-5` /
`claude-sonnet-5`) for its `citations` API. Sending each retrieved product
as a `document` content block with `citations: {enabled: true}` returns a
response where each answer segment links to the exact source document and
the exact quoted span of text that supports it. The API computes and
verifies this itself; the model cannot fake a citation.

Gemini has no equivalent for documents supplied inline per request. Its
closest feature, File Search, requires pre-indexing a whole corpus into a
persistent store and lets Gemini run its own retrieval internally, which
conflicts with this project's hybrid retrieval/RRF pipeline (see the
package doc on `internal/llm/gemini/client.go` for the full reasoning). The
current workaround, implemented in that file's `GenerateAnswer`, is a JSON
schema requiring `{"answer": "string", "cited_product_ids": ["string"]}`,
with the model self-reporting which products it used. The code validates
that the reported IDs are among the candidates it received, which rules
out hallucinated IDs that were never offered. Nothing verifies the claim
itself: whether the answer text relied on that product. So the `cited`
flag on each product in the `/query` response means "the model's
self-report, filtered for validity," not "API-verified."

**If citation accuracy becomes important** (for example, if this moves
from a demo toward something where incorrect grounding has real
consequences), swap `internal/llm/gemini` for an `internal/llm/claude`
implementation of the same `llm.Generator` interface. Nothing else in the
pipeline needs to change; that is the point of the interface. Budget for
Claude's cost at that point (no free tier, roughly $1-10 per million
tokens depending on model).

DeepSeek was also considered and ruled out. It has no free tier: it's
cheap, pay-as-you-go from the first token, not free. It also has no
embeddings API and the same citation-verification gap as Gemini.
