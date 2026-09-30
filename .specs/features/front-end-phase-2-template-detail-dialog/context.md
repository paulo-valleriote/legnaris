# Contexto — Fase 2 · Detalhe do template e diálogo de criação (discussão com o mantenedor)

Decisões capturadas durante o Specify desta spec (2026-09-30). Ver também `../front-end-phase-2-catalog/context.md` (decisões 1, 4 e 5 valem para as três specs da fase).

| # | Área | Decisão | Alternativas descartadas |
| --- | --- | --- | --- |
| 1 | Escopo do diálogo na Fase 2 | Resumo do template + campo de nome + botão "Create sheet" **desabilitado** com tooltip | Só o resumo (Tela 3 incompleta vs wireframe); diálogo funcional (`POST /sheets` — criação é Fase 4) |
| 2 | Regra do resumo por prefixo | Cada chave de topo de `fields` = linha; grupo conta folhas descendentes (recursivo), folha de topo conta 1 sob o próprio nome | Bucket "general" para folhas de topo (inventava rótulo); contar só chaves do wireframe (hardcode — trai a promessa de UI dirigida por schema) |
| 3 | Quando buscar o detalhe | A cada abertura do diálogo, sem cache entre aberturas | Cache por id (exigiria política de invalidação; adiado para o UI Engine, Fase 3) |
| 4 | Campo de nome | Vazio, com placeholder de exemplo localizado | Valor default pré-preenchido (geraria nome inventedado na Fase 4) |

Fontes consultadas: `docs/TDD-desktop-mvp.md` (wireframe Tela 3), `desktop/AGENTS.md` (diálogo é caso legítimo de modal; ação primária desabilitada explica via tooltip), contrato `GET /templates/:id` (`test/fixtures/template_detail.json`, árvore folha/grupo da Fase 1).
