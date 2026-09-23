# AGENTS.md

## Coding
- SEMPRE escreva código em INGLÊS
- SEMPRE prefira documentação acima de comentários no código

## Testing
- SEMPRE use table-tests
- Arquivos de testes devem sempre seguir o padrão <arquivo_original>_test.go
- NUNCA crie varios arquivos de teste para testar um arquivo especifico, apenas <arquivo_original>_test.go
- NUNCA apague, modifique ou manipule testes pré-existentes. Se necessário alguma alteração por mudança de comportamento do objeto testado PEÇA PERMISSÂO antes de alterar
- SEMPRE use mocks para interfaces;
- NAO DUPLIQUE CODIGO, use os mocks já existentes;
- Padrão de testes: testify (`assert`/`require`/`mock`).
- Mocks são gerados por mockery (tool dependency do Go): `make mocks` (= `go tool mockery`, config em `.mockery.yaml`). Regenere sempre que uma interface mudar. Ficam em `<pkg>/mocks` (`shared/logger/mocks`, `internal/sheet/mocks`, `internal/sheet/template/mocks`) e usam a expecter API: `mocks.NewMockX(t)` + `.EXPECT()`/`.On()`.

## Comandos
- `make up` / `make down` — sobe/desce o MongoDB local via docker compose (porta 27017, root/password). A API não sobe sem o Mongo rodando.
- `go run ./cmd` — sobe a API. A porta 8080 está hardcoded em `cmd/api.go` (`e.Start(":8080")`); a variável `PORT` é lida do env mas não é usada no start.
- `go test ./...` — todos os testes; um pacote/teste: `go test ./internal/sheet/template -run TestBuildDag`.
- `make fmt` — `go fmt ./...`. Não há linter nem CI configurados no repo.
- O Makefile faz `-include .env`. Os defaults de env ficam em `shared/env/env.go` (`DATABASE_URI` default já aponta para o Mongo do compose).

## Arquitetura
- `cmd/api.go` é o único entrypoint. Bootstrap: logger → env → Mongo → formula engine → DAGs em memória → **seed dos templates default no Mongo a cada boot** (`template.FantasyV1Seed`) → router Echo.
- `internal/app`: wiring/DI (`NewModules`), rotas (`NewRouter`), controllers e o `HTTPErrorHandler`, que converte `shared/apperr.Error` em JSON `{error: {code, message}}`. Convenção: services retornam `apperr.Error`; não trate HTTP dentro de service/repository.
- `internal/sheet`: fichas são documentos flexíveis (`map[string]any`) no Mongo. O subpacote `internal/sheet/template` é o núcleo do produto: `FieldSchema` (tipo, `Formula`, `DependsOn`, editabilidade, default) e o DAG (ordenação topológica de Kahn, detecção de ciclos, recálculo em cascata).
- `internal/engine`: engine de fórmulas sobre `expr-lang` com funções custom `floor` e `max`.
- Padrão por módulo: interfaces `Repository`/`Service` + `ServiceImpl` + controller HTTP. O database Mongo está hardcoded como `legnaris` em `NewModules`.
- `internal/user` é stub; `OwnerID` em `Sheet` está comentado — auth/ownership são trabalho futuro (escopo em `docs/TDD.md`).
- `docs/TDD.md` (PT-BR) é a fonte de verdade de produto e roadmap.
