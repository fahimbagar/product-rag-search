# Other candidates

- **Auth hardening**: `/ingest` is gated by a single static bearer token
  (`ADMIN_TOKEN`); fine for a demo, not for multi-tenant or production use.
- **Context relevance/precision**: the retrieval eval measures recall (did we
  fetch the right products) but not precision (are the *other* candidates
  that got pulled in actively irrelevant noise) — a distinct RAG metric worth
  adding if retrieved-but-irrelevant products become a problem.
- **Richer input categorization**: `intent.Intent` collapses everything
  that isn't a product query into one `out_of_scope` bucket (greetings,
  spam, genuinely unanswerable-but-in-domain questions all get the same
  label/handling). Splitting these out would let "appropriate refusal rate"
  be measured separately from general answer quality, rather than folded
  into intent-classification accuracy.
