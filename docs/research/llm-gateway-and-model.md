# LLM gateway and model for Sheet generation from a concept

Status: research (LGN-170, parent LGN-169). Not a decision record. Prices and
policies were read on 2026-11-12 and change often; re-check before go-live.

## Question

Which gateway (OpenRouter or OpenCode) and which model should Legnaris use to
turn a natural-language character concept into a Sheet for the `fantasy_v1`
Template, and what does one generation cost?

## Recommendation

- **Model:** DeepSeek Flash (the V4.1 Flash model; the API id is `deepseek-flash`,
  legacy `deepseek-v4-flash` now routes to `DeepSeek-V4.1-Flash-1017` and is billed
  at the Flash price). It supports JSON output and function calling on the
  official API.
- **Gateway:** OpenRouter, with provider routing pinned to endpoints that do not
  retain or train on data: `provider: { zdr: true, data_collection: "deny",
  require_parameters: true }`. Send a `json_schema` `response_format` with
  `strict: true`.
- **Fallback gateway:** OpenCode Console (`https://opencode.ai/inference/openai/v1/chat/completions`),
  which states zero-retention and US/EU hosting for its models.
- **Do not call the DeepSeek first-party API directly** for user concepts. Its
  terms and privacy policy allow training on inputs and outputs and store data in
  the PRC.
- **Validate in code regardless of gateway.** Parse the output against the editable
  field list, reject unknown keys, and retry once on failure. Schema enforcement is
  not uniform across providers.
- **Estimated cost:** about USD 0.001 to 0.005 per generation (see Cost).

## Template size (fantasy_v1)

Measured with a temporary test that walked `fantasyV1Template.Fields`
(`backend/internal/sheet/template/seeders.go`, then removed):

| Metric | Value |
|---|---|
| Total leaves | 130 |
| Editable leaves (numField 24 + textField 9 declared, plus loop-generated) | **93** |
| Derived leaves (calcField, not editable) | 37 |
| Full tree JSON size | ~43 KB (about 11k tokens, not sent to the model) |

Only the 93 editable leaves need to go to the model, as a JSON schema of
key, type and label per field.

## Model and gateway facts

### DeepSeek (first-party) — https://api-docs.deepseek.com/quick_start/pricing

| Model | Context | Input cache hit | Input cache miss | Output |
|---|---|---|---|---|
| deepseek-flash | 1M | off-peak $0.003 / peak $0.006 | off-peak $0.15 / peak $0.30 | off-peak $0.60 / peak $1.20 |
| deepseek-v4-pro | 1M | off-peak $0.022 / peak $0.044 | off-peak $0.66 / peak $1.32 | off-peak $1.98 / peak $3.96 |

- Max output 384K. JSON output and tool calls supported on both models. Vision only on flash.
- Off-peak is half of peak. Peak is 01:00–04:00 and 06:00–10:00 UTC, Monday to Friday, excluding Chinese public holidays.
- Legacy `deepseek-v4-flash` is "retired" and routed to V4.1-Flash-1017. No retirement date is given on the page.
- Concurrency limit: 2500 for flash.

### DeepSeek terms and privacy (first-party only)

- Open Platform ToS, section 4.2(3): inputs and outputs may be used for "training other models (such as model distillation)". https://cdn.deepseek.com/policies/en-US/deepseek-open-platform-terms-of-service.html
- Output rights: section 4.2(2) assigns "any rights, title, and interests—if any—in the Outputs" to the user. Same document.
- Privacy policy: inputs may be used to "train and improve our technology"; data is "stored in People's Republic of China"; no fixed retention period ("as long as you have an account"); training opt-out by email to privacy@deepseek.com only. https://cdn.deepseek.com/policies/en-US/deepseek-privacy-policy.html

### OpenRouter — https://openrouter.ai/deepseek/deepseek-v4-flash

- Model is listed as `deepseek/deepseek-v4-flash` (V4 Flash 0423). The page shows input and output prices only, not cache-read prices. Lists `response_format` and `tools` among accepted parameters.
- Provider prices (input/output per 1M) range from $0.05/$0.45 (Relace) to $0.21/$0.56 (Azure). The page does not show a data policy or ZDR flag per provider and says the model is not available via EU in-region routing.
- Structured outputs: `response_format` with `type: "json_schema"`. Support is per endpoint, so check the `structured_outputs` parameter per provider. `strict: true` is best-effort: "exact compliance is not guaranteed on every endpoint." https://openrouter.ai/docs/guides/features/structured-outputs
- Routing: `zdr: true` routes only to ZDR endpoints (Safety Retention endpoints are also eligible unless the account is set to "ZDR only"). `data_collection: "deny"` uses only providers that do not collect user data. `require_parameters: true` drops endpoints that cannot take the request, including the schema. https://openrouter.ai/docs/guides/routing/provider-selection
- Privacy: OpenRouter stores prompts only if an opt-in logging or data-sharing setting is on (both off by default). Prompts are not used for product improvement unless opted in. OpenRouter does not state whether it trains its own models. https://openrouter.ai/docs/guides/privacy/data-collection
- Prior-generation reference (not the target): DeepSeek V3.2 on OpenRouter, cache read $0.0216/M. Source: https://openrouter.ai/deepseek/deepseek-v3.2-20251201 (from search; not opened).

### OpenCode Console — https://opencode.ai/docs/zen/

The page is titled "OpenCode Console"; no page named "Zen" was found.

| Model | Input | Cached input | Output |
|---|---|---|---|
| DeepSeek V4.1 Flash | $0.30 | $0.006 | $1.20 |
| DeepSeek V4 Pro | $1.74 | $0.145 | $3.48 |
| DeepSeek V4.1 Flash Free | free | free | free |

- Models hosted in the US and EU. Providers "generally follow a zero-retention policy and do not train on your data". The free V4.1 Flash is described as zero-retention and not used for training.
- Endpoint: `https://opencode.ai/inference/openai/v1/chat/completions` (package `@ai-sdk/openai-compatible`). Model list at `https://opencode.ai/inference/v1/models`.
- V4 Flash is deprecated with a date of 2026-10-26.
- Structured output, JSON schema and tool-call support for DeepSeek models is not stated.
- Rate limits for DeepSeek models are not stated.

Note: OpenCode's input, cached-input and output prices match DeepSeek's peak prices. The page does not say whether OpenCode follows DeepSeek's off-peak schedule. Treat the figures as the page's, not verified against an invoice.

## Cost per generation

Assumptions (all estimates, to be measured):

- Schema: 93 editable fields as a JSON schema with type and label, about 30 tokens each: **~2,800 input tokens**, cacheable as a prefix.
- Concept prompt: **~200 tokens**.
- Output: 93 values as JSON, **~900 tokens**. Thinking mode is on by default; reasoning tokens bill as output. Assume **~2,000 reasoning tokens** as a planning figure.
- Prefix cache hit on most calls after the first.

Per generation, DeepSeek flash prices:

| Scenario | Input | Output | Cost |
|---|---|---|---|
| Peak, no cache | 3,000 × $0.30/M = $0.0009 | 2,900 × $1.20/M = $0.0035 | **~$0.0044** |
| Peak, schema cached | 200 × $0.30/M + 2,800 × $0.006/M = $0.00008 | $0.0035 | **~$0.0036** |
| Off-peak, schema cached | half of peak | half of peak | **~$0.0018** |
| Thinking off (output 900) | ~$0.0002 + $0.0011 | | **~$0.0013** |

Range: **USD 0.001 to 0.005 per generation**. At 10,000 generations/month, that is USD 10 to 50. The cache discount is small here because the schema is a small share of input.

## Open risks

1. **Schema enforcement.** Not uniform across OpenRouter endpoints; `strict` is best-effort. Validation and a retry are required.
2. **Data policy per provider.** The OpenRouter page does not list a per-provider policy or a ZDR flag for flash. Confirm which endpoints return `zdr` eligibility before launch (`GET /api/v1/models/deepseek/deepseek-v4-flash/endpoints` or the Providers tab).
3. **DeepSeek first-party policy.** Training on inputs and PRC storage. Avoid unless the user explicitly accepts it.
4. **Model churn.** Names changed three times within the window (v4-flash, v4.1-flash-1017, deepseek-flash). Pin the id and watch for deprecations; OpenCode lists a 2026-10-26 deprecation for V4 Flash.
5. **Prices and peak schedule** vary by gateway and change. The cost estimate depends on the thinking-token assumption; measure on a sample of 50 concepts.
6. **Latency and rate limits** were not measured. Peak hours may add latency; test from the production region.
7. **OpenRouter terms of service** and **OpenCode terms** were not read for commercial-use or indemnity clauses. Only DeepSeek's terms were checked.
8. **Caching on OpenRouter** for flash was not confirmed (V3.2 cache read is listed, flash is not).
9. **Go client.** Both gateways expose an OpenAI-compatible chat completions endpoint, so a generic OpenAI-compatible Go client (e.g. `github.com/openai/openai-go`) should work with a base-URL change. Not tested.

## Sources

- DeepSeek pricing: https://api-docs.deepseek.com/quick_start/pricing
- DeepSeek ToS: https://cdn.deepseek.com/policies/en-US/deepseek-open-platform-terms-of-service.html
- DeepSeek privacy policy: https://cdn.deepseek.com/policies/en-US/deepseek-privacy-policy.html
- OpenRouter flash model page: https://openrouter.ai/deepseek/deepseek-v4-flash
- OpenRouter structured outputs: https://openrouter.ai/docs/guides/features/structured-outputs
- OpenRouter provider routing: https://openrouter.ai/docs/guides/routing/provider-selection
- OpenRouter data collection: https://openrouter.ai/docs/guides/privacy/data-collection
- OpenCode Console docs: https://opencode.ai/docs/zen/
- Repo template source: `backend/internal/sheet/template/seeders.go` (`fantasyV1Template`, `numField`, `textField`, `calcField`)
