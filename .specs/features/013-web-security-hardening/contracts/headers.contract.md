# Contract: Security Headers & CSP (Frontend Response Headers)

**Source**: `docs/plan-web-pwa.md` §4, §9, §10; `F9-T001`, `F9-T002`, `F9-T005`, `F9-T010`.

**Localização**: Proxy/Traefik do `app.legnaris.com.br` (ou `vite.config.ts` se configurado para preview, mas a produção é via proxy HTTP).

**Headers obrigatórios (cada resposta)**:

```
Content-Security-Policy: default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self' https://api.legnaris.com.br; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; require-trusted-types-for 'script'; report-to csp-endpoint
Reporting-Endpoints: csp-endpoint="https://api.legnaris.com.br/csp-report"
Strict-Transport-Security: max-age=63072000; includeSubDomains; preload
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Permissions-Policy: ...
Cross-Origin-Opener-Policy: same-origin
Cross-Origin-Resource-Policy: same-origin
```

**Nota**: `report-uri` (legado) é usado como fallback; `report-to` + `Reporting-Endpoints` é o moderno.

**Rollout**: `Content-Security-Policy-Report-Only` (fase `report-only`) → `Content-Security-Policy` (enforce) após coleta (`F9-T008`).

**Teste (`F9-T010`)**: integração que faz `curl -I https://app.legnaris.com.br` e confirma presença de todos os headers; falha se algum sumir.
