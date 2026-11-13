---
status: accepted
---

# Character generation is an async draft the player accepts

A player generates a character Sheet from a short concept text. `POST /generations` stores a `generation` document, publishes a job to RabbitMQ and answers `202 {id}`. A worker asks the LLM for values that fit a strict JSON Schema derived from the Template, validates them and stores them on the same document. The client polls `GET /generations/{id}` and shows a preview. The player can edit values locally, and nothing becomes a Sheet until `POST /generations/{id}/accept`. The LLM only supplies values for existing editable leaves. It never writes formulas or keys, and its output is treated as untrusted data. Gateway and model come from LGN-170 and LGN-174: OpenCode with DeepSeek Flash, never the DeepSeek first-party API.

## Considered Options

- **Synchronous request:** a generation takes 10–40 s with reasoning, which is too long for a mobile/PWA request held open through proxies.
- **Mongo-only queue (worker claims `queued` documents by polling):** simpler, but it would leave the project with two job mechanisms when RabbitMQ is already deployed. RabbitMQ also routes by kind through a topic exchange when NPC/Monster arrive.
- **A separate outbox collection, as email does:** the `generation` document already records that a job is owed (`status=queued`), so a sweeper can republish from it directly.
- **Persist the generated Sheet immediately:** this would take a Sheet slot and leave a Sheet the player never asked for whenever they discard or regenerate.

## Decisions

- **Input:** `{template_id, kind, concept}`. In the MVP, `kind` must be `character`. `concept` is 10–1000 characters, and generated text follows its language.
- **Gates before the quota is reserved, in order:**
  1. A read-only check of the Sheet limit for the kind (LGN-183) → `SHEET_LIMIT_REACHED` 409.
  2. At most one `queued`/`running` generation per user → `GENERATION_IN_PROGRESS` 409.
  3. Quota reservation (LGN-182).
- **Output schema:**
  - It is nested like the Template's groups, with `additionalProperties: false`, and covers only the leaves marked `Generatable` (so `rules.*` is excluded).
  - Each `ExclusiveGroup` becomes one enum property, which code expands back into its 0/1 flags.
  - `FieldSchema` gains `Generatable`, `Min`, `Max` and `MaxLength`. In the MVP only the generation validator enforces them.
- **Validation:**
  1. Strict JSON parse.
  2. Types, keys, ranges and lengths checked in Go (the DAG does not check value types).
  3. Exclusive groups expanded.
  4. Template seeded and `NormalizeAndRecalculate` run, as `PreviewTemplate` does.

  On any failure the worker retries up to 2 times, sending the errors back to the model; these retries are free. If all three attempts fail, the job ends `failed` and the quota is refunded.
- **Transport failures:** network errors, 5xx and 429 from the gateway get one in-process retry with a short backoff. RabbitMQ delay tiers are not used, because the player is waiting. The email circuit breaker is reused: when it is open, the job fails fast with `provider_unavailable` and the quota is refunded.
- **Messaging:**
  - Publishing: topic exchange `generation`, routing key `generation.<kind>.requested`, queue `generation.requests`, DLQ `generation.dead`. The API publishes with publisher confirms. If the publish fails, the API still answers 202, and a sweeper in `relay` republishes `queued` documents older than 10 s.
  - Consuming: the consumer lives in the existing `worker` binary with prefetch 4. Claims are idempotent (`findOneAndUpdate` `queued→running` plus a 90 s lease), and the message is acked only after the terminal state is written. When a lease expires, the job becomes `failed` and the quota is refunded.
- **States:** `queued | running | succeeded | failed | accepted`. The document expires by TTL after 24 h. An expired generation, or one owned by another user, returns 404.
- **Client contract:** the client polls every 5 s and gives up after 120 s. The routes have their own rate-limit namespace: `GET /generations/:id` 30/min and `POST /generations` 5/min per user.
- **Preview and accept:**
  - Tweaks are local edits, and they are free.
  - Regenerate is a new `POST /generations` and consumes 1 from the quota.
  - `POST /generations/{id}/accept {sheet_name, values}` validates `values` through the normal DAG path, the same one PATCH uses (not the generation ranges). It then reserves the Sheet slot with LGN-183's conditional `$inc` (409 if another create won the race) and creates the Sheet through `sheet.Service.CreateWithValues`. A second accept returns 200 with the same `sheet_id`.
- **Boundaries:**
  - `llm.Client` (port, in `internal/llm`) with the OpenCode adapter (`internal/llm/opencode`, OpenAI-compatible HTTP), and `generation.Publisher`. Both are injected through constructors and mocked with mockery.
  - Prompt and schema builders are pure functions in the `generation` package.
  - Config: `LLM_PROVIDER` (default `opencode`), `LLM_BASE_URL`, `LLM_API_KEY`, `LLM_MODEL`, `LLM_TIMEOUT`.
- **Prompt:**
  - The system prompt is fixed, in English, and describes the Template's labels and options.
  - The concept goes in the user message, delimited and declared to be a description, not instructions. The defense against prompt injection is the strict schema plus validation.
  - `max_tokens` is about 6k.

## Consequences

- Concept and output live only in the `generation` document, for at most 24 h, and are never logged. Logs record id, status, attempts, latency and tokens.
- Generation depends on LGN-183 (Sheet kind and per-Plan limits) and LGN-182 (daily quota). A Free user has a quota of 0 and cannot generate.
- Swapping the gateway or model means a new `llm.Client` adapter and a configuration change; the service does not change.
- Tests:
  - The `generation` service runs on mocks of `llm.Client` and `Publisher`.
  - The adapter is tested with `httptest`.
  - A test against the real gateway self-skips when `LLM_API_KEY` is unset.
