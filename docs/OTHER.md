# Other candidates

- Auth hardening: `/ingest` is gated by a single static bearer token
  (`ADMIN_TOKEN`), which is fine for a demo but not for multi-tenant or
  production use.
- Context relevance/precision: the retrieval eval measures recall (did
  the pipeline fetch the right products) but not precision (are the other
  candidates it pulled in irrelevant noise). This is a distinct RAG metric
  worth adding if retrieved-but-irrelevant products become a problem.
- Richer input categorization: `intent.Intent` collapses everything
  that isn't a product query into one `out_of_scope` bucket. Greetings,
  spam, and unanswerable-but-in-domain questions all get the same
  label and handling. Splitting these out would let "appropriate refusal
  rate" be measured separately from general answer quality, instead of
  folded into intent-classification accuracy.
