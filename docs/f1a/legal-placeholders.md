# F1a: legal placeholders to fill (LGN-228)

Epic: LGN-221. Go-live is blocked until legal fills every item below and the
consent version is bumped if the text changes.

Files that carry placeholders (the three consent copies must stay identical):

- `docs/legal/llm-consent.json` (source)
- `backend/internal/llmconsent/llm-consent.json` (embedded in the API)
- `web/src/generate/llm_consent.json` (shown by the web client)
- `docs/legal/privacy-policy.en.md`
- `docs/legal/privacy-policy.pt-BR.md`

| Placeholder (EN / pt-BR) | What to provide |
|---|---|
| `CONTROLLER_LEGAL_NAME` / `NOME_JURIDICO_CONTROLADOR` | Legal name of the Legnaris controller |
| `CONTROLLER_CNPJ_OR_EQUIVALENT` / `CNPJ_OU_EQUIVALENTE` | Company registration number |
| `CONTROLLER_ADDRESS` / `ENDERECO_CONTROLADOR` | Registered address |
| `DATA_PROTECTION_OFFICER_NAME_OR_ROLE` / `NOME_OU_FUNCAO_DO_ENCARREGADO` | DPO name or role |
| `DATA_PROTECTION_CONTACT_EMAIL` / `EMAIL_DO_ENCARREGADO` | DPO contact |
| `PRIVACY_CONTACT_EMAIL` / `EMAIL_CONTATO_PRIVACIDADE` | General privacy contact |
| `MAIN_PRIVACY_POLICY_URL`, `PRIVACY_POLICY_URL` / `URL_POLITICA_PRINCIPAL` | Public URL of the policy |
| `OPENROUTER_LEGAL_NAME`, `OPENROUTER_COUNTRY` / `NOME_JURIDICO_OPENROUTER`, `PAIS_OPENROUTER` | OpenRouter entity and country |
| `SERVING_PROVIDER`, `SERVING_PROVIDER_LEGAL_NAME`, `SERVING_PROVIDER_COUNTRY` / `NOME_JURIDICO_SERVING_PROVIDER`, `PAIS_SERVING_PROVIDER` | The provider that actually serves the model. Depends on LGN-227 (see below) |
| `PROCESSING_COUNTRIES` / `PAISES_DE_PROCESSAMENTO` | Countries where the data is processed |
| `RETENTION_AT_PROCESSORS_CONFIRMED_BY_LEGAL` / `RETENCAO_NOS_OPERADORES_CONFIRMADA_PELO_JURIDICO` | Confirmation of retention at processors under ZDR |
| `REVOKE_SCREEN_PATH`, `REVOKE_API_PATH` / `CAMINHO_TELA_REVOGACAO`, `CAMINHO_API_REVOGACAO` | Revocation screen and API paths (engineering fills these) |
| `DATE` / `DATA` | Effective date |
| `SQUARE_BRACKETS` / `COLCHETES` | Prose mention of the bracket convention; remove once all values are filled |

Blocker: `SERVING_PROVIDER` cannot be named until LGN-227 shows which
endpoint satisfies `zdr: true`, `data_collection: "deny"`, `ignore: ["DeepSeek"]`.

After filling: bump the consent version constant, keep the three JSON copies
byte-identical, and re-run `go test ./backend/internal/llmconsent/...` and the
web tests.
