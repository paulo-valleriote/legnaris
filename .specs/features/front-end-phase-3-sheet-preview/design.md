# [Front End] Fase 3 — Preview read-only no diálogo de criação Design

**Spec**: `.specs/features/front-end-phase-3-sheet-preview/spec.md`
**Status**: Approved (amend UAT 2026-10-01)

---

## Architecture Overview

O diálogo de criação passa a ter **coluna única** dentro do mesmo `AlertDialog`: identidade do template (`nome | id` com divisor hairline), campo de nome e, **abaixo**, um **card de preview** que renderiza `SheetForm` com os defaults do schema e `onChanged: null`. Não há provider novo, não há requisição nova e não há mudança no shell: o card observa o **mesmo** `templateDetailControllerProvider(templateId)` da Fase 2, então as duas áreas compartilham o único `GET /templates/:id` da ativação (PRV-11) e os estados loading/erro/vazio vivem dentro do card (PRV-04, PRV-07…09). O card tem cabeçalho **"Read-only preview"**, altura máxima com **barra de rolagem sempre visível** (PRV-06, PRV-10). O resumo textual da Fase 2 sai do diálogo — e junto com ele `template_summary.dart` (amend da spec da Fase 2).

**Nota de pesquisa (SDK verificado, não presumido):** `AlertDialog` envolve o conteúdo em `IntrinsicWidth` (`flutter/lib/src/material/dialog.dart:1354`), que **aperta a largura para a largura intrínseca do filho** quando a restrição recebida não é tight (`flutter/lib/src/rendering/proxy_box.dart:712`). Consequências para este layout, verificadas no código do SDK:

1. `ConstrainedBox(maxWidth:)` **não** bastaria: a largura intrínseca do `Row` inclui a do `TextField` — o diálogo mudaria de largura enquanto o usuário digita o nome.
2. Um `SizedBox(width: token)` é **tight**, então o `IntrinsicWidth` o respeita (`tighten(hasTightWidth ? null : intrinsic)`) → largura fixa e estável, e o `Row` com `Expanded` recebe restrição finita.
3. Em janela estreita o `tighten` é limitado pelo `maxWidth` do pai (clamped) → o diálogo nunca estoura a viewport.

```mermaid
graph TD
    CS[CatalogScreen] -->|showDialog| D[CreateSheetDialog]
    D -->|watch family templateId| P[TemplateDetailController - autoDispose family]
    P -->|GET /templates/:id| API[(Backend)]
    D -->|identidade + nome| ID[nome | id + TextField do nome]
    D -->|abaixo| C[_PreviewCard privado]
    C -->|state: loading / erro+Retry / no-fields| ST[TemplateDetailState]
    C -->|sucesso| SF[SheetForm - onChanged null]
    SF --> FC[SheetFieldControl - read-only]
    C -->|seedDefaults| SD[engine/sheet_defaults]
    D -->|sem mudanca| ACT[Create sheet desabilitado + Close]
```

### Cadeia de layout (verificada)

```
AlertDialog.content                                    // IntrinsicWidth (dialog.dart:1354)
 └─ SizedBox(width: AppSizes.createSheetDialogWidth)   // tight 560 → largura estável
     └─ Column(crossAxisAlignment: start)
         ├─ identidade (Row: nome + divisor hairline + id)
         ├─ label "Sheet name" + TextField
         └─ Card do preview (largura total, header "Read-only preview")
             └─ ConstrainedBox(maxHeight: AppSizes.previewCardMaxHeight)
                 └─ Scrollbar(thumbVisibility: true, controller)
                     └─ SingleChildScrollView(controller)
                         └─ estado (loading/erro+Retry/vazio) ou SheetForm(..., onChanged: null)
```

---

## Code Reuse Analysis

### Existing Components to Leverage

| Component | Location | How to Use |
| --- | --- | --- |
| `SheetForm(fields, data, onChanged)` | `desktop/lib/engine/sheet_form.dart` | Corpo do card — seções + linhas; `onChanged: null` torna tudo inerte (PRV-06) |
| `SheetFieldControl` (`_formatValue`) | `desktop/lib/engine/field_control.dart` | Controle read-only por regra do schema; `10.0` → `10`, toggle desabilitado no estado do default |
| `buildSections` / `fieldDisplayName` | `desktop/lib/engine/field_grouping.dart` | Derivação de seções e nomes (PRV-01…03) — já testada na Fase 3 (RND-01…05) |
| `valueAtPath` | `desktop/lib/api/models/sheet.dart:64` | Leitura do valor por caminho pontilhado dentro do mapa semeado |
| `SheetScreen._seedDefaults` / `_setValue` | `desktop/lib/sheet/sheet_screen.dart:147` | **Extraído** para `engine/sheet_defaults.dart` e compartilhado (ver Components) |
| `templateDetailControllerProvider` | `desktop/lib/catalog/template_detail_provider.dart` | Mesmo estado da Fase 2 — sem provider novo, sem cache |
| `failureCopy` / `actionRetry` | `desktop/lib/api/api_error.dart`, l10n | Mensagem mapeada + Retry dentro do card (PRV-07/08) |
| Tokens de tema | `desktop/lib/theme/tokens.dart` | Três tokens novos (ver Data Models); nenhum número mágico nos widgets |
| `MockClient` harness | `desktop/test/catalog/create_sheet_dialog_test.dart` | Mesmo transporte fake; contagem de chamadas para PRV-11/12/13 |
| `AppLocalizations` | `desktop/lib/l10n/` | Chave nova `createSheetPreviewLabel`; `flutter gen-l10n` (`generate: true`) |

### Integration Points

| System | Integration Method |
| --- | --- |
| `CreateSheetDialog` → provider | `ref.watch(templateDetailControllerProvider(widget.templateId))` — inalterado |
| Card → renderer | `SheetForm(fields: template.fields, data: seedDefaults(template.fields), onChanged: null)` |
| `ProviderScope` | Testes sobrescrevem `apiClientProvider` com `ApiClient` sobre `MockClient` (padrão da Fase 2) |
| Fase 2 (diálogo) | Mesmo arquivo ganha o preview **abaixo** do nome; navegação CAT-15/16/17 intocada |

---

## Components

### `lib/catalog/create_sheet_dialog.dart` (reestruturado)

- **Purpose**: Tela 3 em coluna única — identidade (`nome | id`) + nome e, abaixo, o card de preview (PRV-01…13).
- **Location**: `desktop/lib/catalog/`
- **Interfaces**:
  - `build` — conteúdo passa a ser `SizedBox(width: AppSizes.createSheetDialogWidth)` (tight 560 — ver nota de pesquisa) → `Column(crossAxisAlignment: start)`: bloco de identidade (Row `nome` + divisor hairline + `templateId`) + label `createSheetNameLabel` + `TextField` (TPL-11, PRV-13) + card do preview abaixo.
  - `templateId` mantém `Key('create-sheet-template-id')` (contrato CAT-15); o divisor hairline só aparece quando o nome é não vazio.
  - `_preview(context, state, localizations)` (privado): `Card` com cabeçalho caption `createSheetPreviewLabel` ("Read-only preview") e `Key('create-sheet-preview')`; corpo = `ConstrainedBox(maxHeight: AppSizes.previewCardMaxHeight)` → `Scrollbar(thumbVisibility: true, controller: _previewScrollController)` → `SingleChildScrollView(controller: _previewScrollController)` com o estado: `isLoading` → `LinearProgressIndicator` + `templateDetailLoading`; `errorMessage != null` → copy mapeada + `OutlinedButton(actionRetry)`; sucesso com `buildSections` vazio → `templateNoFields`; sucesso → `SheetForm(..., onChanged: null)`.
  - `State`: `ScrollController _previewScrollController` (init/dispose) compartilhado entre `Scrollbar` e `SingleChildScrollView`.
  - Removido: `_detail` (resumo textual) e o import de `template_summary.dart`.
- **Dependencies**: `templateDetailControllerProvider`, `SheetForm`, `seedDefaults`, `failureCopy` (via estado), tokens, l10n.
- **Reuses**: mesmos estados de loading/erro da Fase 2 (agora dentro do card), tokens, padrão de erro inline.

### `lib/engine/sheet_defaults.dart` (novo)

- **Purpose**: semear os valores base do preview/Tela 2 a partir do `FieldSchema` e escrever valores por caminho pontilhado — **um único mecanismo** para as duas superfícies.
- **Location**: `desktop/lib/engine/`
- **Interfaces**:
  - `Map<String, Object?> seedDefaults(FieldGroup fields)` — itera `buildSections` e grava `schema.defaultValue` de cada folha no mapa aninhado (`stats → { constitution: 10 }`); `fields` vazio → mapa vazio.
  - `void setPathValue(Map<String, Object?> data, String path, Object? value)` — cria os nós intermediários e atribui o valor (extraído de `SheetScreen._setValue`).
- **Dependencies**: `buildSections` (`field_grouping.dart`).
- **Reuses**: `SheetScreen._seedDefaults`/`_setValue` deixam de existir como cópias privadas; `valueAtPath` continua sendo o leitor em `api/models/sheet.dart`.

### `lib/sheet/sheet_screen.dart` (refactor sem mudança de comportamento)

- **Purpose**: consumir `seedDefaults`/`setPathValue` compartilhados em vez das cópias privadas.
- **Interfaces**: inalteradas (RND-14…19 seguem válidos; os testes existentes não mudam de asserção).

### `lib/theme/tokens.dart` (dois tokens)

- `AppSizes.createSheetDialogWidth = 560` — `SizedBox` **tight** no conteúdo do diálogo: sem ele o `IntrinsicWidth` do `AlertDialog` aperta a largura na intrínseca do `Column`/`TextField` e o diálogo mudaria de largura enquanto o usuário digita o nome (ver nota de pesquisa); em janela estreita o valor é limitado pelo pai, sem overflow.
- `AppSizes.previewCardMaxHeight = 420` — limite do card com scroll interno e barra sempre visível (PRV-10); ~20 folhas de `fantasy_v1` não estouram a viewport.
- **Removido no amend UAT:** `AppSizes.previewCardWidth = 360` (a largura fixa da coluna do card deixou de existir com a coluna única; o card ocupa a largura do conteúdo).

### `lib/l10n/app_en.arb`

- **Novo**: `createSheetPreviewLabel` ("Read-only preview") — cabeçalho do card, estilo caption, com `Key` de teste própria.
- **Remoção**: nenhuma chave fica órfã (`templateNoFields`, `templateDetailLoading`, `actionRetry` passam a ser usadas pelo card).

### `lib/catalog/template_summary.dart` (removido) + `.specs/features/front-end-phase-2-template-detail-dialog/` (amend)

- `summarizeFields`/`FieldSummary` e `desktop/test/catalog/template_summary_test.dart` são apagados (código morto após PRV-05); `fieldDisplayName` **não** é afetada (vive em `engine/field_grouping.dart`).
- A spec da Fase 2 ganha nota de amend e TPL-02/03/04/05 são reescritos (precedente do amend de UAT); as três linhas de Assumptions sobre o resumo recebem a marca "Supersedido pelo amend LGN-100".

---

## Data Models

### Tokens (novos)

```dart
abstract final class AppSizes {
  static const double createSheetDialogWidth = 560;
  static const double previewCardMaxHeight = 420;
}
```

### Mapa de valores base (efêmero)

```dart
// seedDefaults(FieldGroup) → { stats: { constitution: 10.0, dexterity: 10.0 }, ... }
Map<String, Object?> seedDefaults(FieldGroup fields);
```

**Relationships**: `TemplateDetailState` (Fase 2) continua sendo o único estado do diálogo; o mapa de defaults é recomputado a cada build do card, vive em memória e não é persistido.

---

## Error Handling Strategy

| Error Scenario | Handling | User Impact |
| --- | --- | --- |
| Transport failure (offline/timeout) | `ApiClient` retenta 1× → `ApiDomainException` → `failureCopy` → mensagem dentro do card + Retry | Diálogo segue aberto, coluna esquerda intacta (PRV-07) |
| 4xx/5xx com envelope válido | Kind mapeado → copy do kind no card | Mensagem + Retry (PRV-07) |
| `NOT_FOUND` | `kind: notFound` → `errorNotFound` | Mesmo caminho inline; nunca fecha (PRV-07) |
| 2xx sem `data` / shape inválido | `ApiParseException` → `errorParse` | Nunca card meio-renderizado (PRV-07) |
| Retry | `onRetry` → `load()` com guarda de single-flight | Exatamente 1 GET por ativação (PRV-08/11) |
| `fields` vazio / só grupos sem folhas | `buildSections` vazio → `templateNoFields` | Linha localizada no card (PRV-09) |
| Fecho do diálogo durante carga | Provider autoDispose + `ref.mounted` (já existente) | Sem "use after dispose" |

---

## Risks & Concerns

| Concern | Location (file:line) | Impact | Mitigation |
| --- | --- | --- | --- |
| `AlertDialog` envolve o conteúdo em `IntrinsicWidth` — largura intrínseca instável (o `TextField` do nome muda a intrínseca ao digitar) e `Expanded` exige restrição finita | `desktop/lib/catalog/create_sheet_dialog.dart:40`, SDK `material/dialog.dart:1354` | Diálogo "pulsando" de largura enquanto se digita, ou `RenderFlex` com largura infinita | `SizedBox(width: createSheetDialogWidth)` **tight** antes do scroll (o `IntrinsicWidth` o respeita e o clampa em janela estreita); teste de largura do diálogo com nome longo digitado |
| Scroll aninhado/limitado: card com altura máxima e scroll interno | `desktop/lib/catalog/create_sheet_dialog.dart` | Card cresce sem limite → diálogo maior que a viewport (PRV-10) | Card com `maxHeight` token + `Scrollbar(thumbVisibility: true)` sobre o scroll interno; teste com fixture de 30+ folhas afirmando barra visível, altura ≤ token e nome/ações alcançáveis |
| `Scrollbar(thumbVisibility: true)` exige um `ScrollController` anexado | `desktop/lib/catalog/create_sheet_dialog.dart` | Assertion em runtime se o controller não for compartilhado com o `SingleChildScrollView` | Um único `ScrollController` no `State`, passado a `Scrollbar` e `SingleChildScrollView`, com `dispose()` |
| Teste existente afirma que `config`/`derived`/`modifiers` são **omitidos** | `desktop/test/catalog/create_sheet_dialog_test.dart:136` | Trocar a regra sem inverter o teste derruba o gate; apagar o teste enfraquece a cobertura | Inverter a asserção (agora **presentes**, PRV-01) na mesma task — nunca remover |
| Cópia privada de seed/set de caminho na Tela 2 | `desktop/lib/sheet/sheet_screen.dart:147` | Dois mecanismos escrevendo o mesmo formato de mapa (viola a consistência do AGENTS.md) | Extrair para `engine/sheet_defaults.dart`; Tela 2 refactorada com testes existentes verdes |
| Remoção do resumo quebra artefatos da Fase 2 | `desktop/lib/catalog/template_summary.dart`, `.specs/features/front-end-phase-2-template-detail-dialog/` | Spec aprovada contradizendo o código; código morto | Task própria: amend na spec da Fase 2 (TPL-02/03/04/05) + deleção de `template_summary*` no mesmo commit |
| Chaves `sheet-section-*`/`sheet-row-*` compartilhadas entre o card e a Tela 2 | `desktop/lib/engine/sheet_form.dart:34` | Ambiguidade de `find` se as duas superfícies montassem juntas | Diálogo (overlay) e Tela 2 (rota empurrada) nunca montam simultaneamente; testes do card rodam com o diálogo só |
| Cobertura dos novos estados limitada ao fixture `fantasy_v1` | `desktop/test/catalog/create_sheet_dialog_test.dart` | Bug de semeadura ou de limite de altura passaria despercebido | Fixtures sintéticas: 30+ folhas, `fields` vazio, grupo sem folhas, prefixo desconhecido, default bool/number/text |

---

## Tech Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Composição do card | Método/coluna privada **dentro** de `create_sheet_dialog.dart` — sem widget compartilhado novo | `desktop/AGENTS.md`: não abstrair diálogos de uso único nem criar camada de base; o card é estrutura local e é testado pelo próprio diálogo |
| Estado do preview | Reuso do `templateDetailControllerProvider` (nenhum provider novo) | As duas colunas são a mesma requisição (PRV-11) e a Fase 2 já define o ciclo on-demand/sem cache (PRV-12) |
| Origem dos valores | `seedDefaults` extraído da Tela 2 e compartilhado | Um único mecanismo de escrita de mapa; o preview e a Tela 2 não podem divergir |
| Largura/altura | Dois tokens (`560`/`420`) em `tokens.dart` | AGENTS.md proíbe número mágico em widget; o limite de altura é a garantia de PRV-10 |
| Sorteio dos estados | Loading/erro/vazio **dentro** do card; o bloco de identidade + nome fica estável | PRV-04/07/09 pedem a área do preview; a identidade/nome não mudam enquanto se digita (PRV-13) |
| Largura do diálogo | `SizedBox(width: 560)` tight em vez de `ConstrainedBox(maxWidth:)` | `AlertDialog` usa `IntrinsicWidth` (`dialog.dart:1354`); só uma restrição tight é respeitada, sem reagir à intrínseca do `TextField` |
| Scroll do card | `Scrollbar(thumbVisibility: true)` com `ScrollController` compartilhado | PRV-10 (amend UAT): a rolagem precisa ser visível, não implícita |
| Rótulo do card | Header "Read-only preview" via `createSheetPreviewLabel` | PRV-06 (amend UAT): o modo somente-leitura é explícito |
| Identidade | `nome | id` na mesma `Row` com divisor hairline | Pedido do mantenedor (UAT); o divisor é a affordance do design system |
| Resumo textual | Removido (`template_summary.dart` + teste), spec da Fase 2 amendada | Decisão do mantenedor: sem duplicar a mesma lista; sem código morto |

> **Project-level decisions:** nenhuma. `.specs/STATE.md` não existe neste repositório e as fases anteriores não registraram decisões `AD-NNN`; as escolhas acima são locais a esta feature.
