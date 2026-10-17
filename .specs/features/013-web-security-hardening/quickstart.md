# Quickstart: Web Security Hardening (F9)

**Pré-requisitos**: `F6B-T001` (cookie httpOnly) e `F6B-T002` (CSRF token) concluídos; `F1-T006` (`injectManifest`) ativo; `F1-T008` (`ci-web.yml`) existente.

1. Confirmar `SEC-*` únicos no `.specs/features/013-web-security-hardening/spec.md`.
2. Executar `bash .specify/scripts/bash/setup-plan.sh --json` (já feito) e preencher `plan.md`.
3. Criar `research.md` (decisões), `data-model.md` (entidades configuracionais), `contracts/` (CSP-report, headers), `quickstart.md`.
4. Escrever `tasks.md` (Passo 5) com tarefas `F9-T001` a `F9-T011`, ordenadas por prioridade (`P1` → `P2` → `P3`).
5. Executar `bash .specify/scripts/bash/check-prerequisites.sh --json --require-spec --require-tasks --include-tasks` (Passo 6 — Analyze) para gerar o relatório de análise.
6. Não implementar sem autorização explícita do usuário (regra `speckit-delivery`).

**Gates locais antes de commit**:
```bash
make fmt
make lint
make vet
make lint-specs
make lint-web
make test-web
make sec-scan  # definido no Makefile (F9-T011)
```
