# Data Model

## Overview

The Web Screens Refactor (F3) is a **presentation-layer only** change. No backend or storage schema changes are involved. The data model therefore remains **identical** to F2 and F1, with one exception: the schema definition (`FieldSchema`) is consumed by the F4 UI Engine to render the screen. F3 simply replaces the visual rendering of that schema with F2 primitives.

## Entities (unchanged from preceding features)

### Template

Represents a reusable schema for a sheet.

| Attribute | Type | Description | Used By |
|-----------|------|-------------|---------|
| `id` | string (UUID) | Canonical lowercase UUID v7 | Catalog listing, sheet open |
| `name` | string | Display name of the template | Catalog header, sheet header |
| `schema` | `FieldSchema[]` | Tree of field definitions | Sheet Form (F4), field rendering |
| `version` | number | Template version (increment) | Sheet open (template refresh) |
| `created_at` | Date | Creation timestamp | Metadata display (optional) |

### Sheet

Represents an instance of a template filled with user data.

| Attribute | Type | Description | Used By |
|-----------|------|-------------|---------|
| `id` | string (UUID) | Canonical lowercase UUID v7 | Sheet open, URL |
| `template_id` | string (UUID) | Reference to Template | Sheet open, save |
| `name` | string | Sheet name (editable) | Sheet header, list navigation |
| `data` | `{ [dottedPath]: value }` | Map of field values keyed by schema path | Sheet edit, preview |
| `updated_at` | Date | Last modification timestamp | Dirty state, version conflict |

### FieldSchema (F4 integration point, read-only for F3)

Defines the structure of a single field.

| Attribute | Type | Description | Used By |
|-----------|------|-------------|---------|
| `key` | string | Dotted path for data storage (e.g., `stats.armorClass`) | Sheet edit, cascade |
| `type` | 'string' \| 'number' \| 'boolean' \| 'array' \| 'object' | Value type | Field control selection |
| `label` | string | User-readable label (localized) | Field label |
| `description` | string \| null | Help text | `FieldDescription` component (optional) |
| `kind` | 'input' \| 'select' \| 'textarea' \| 'checkbox' \| 'radio' \| 'switch' | Input kind | Control component selection |
| `options` | `LabeledValue[]` | Select choices | Select control rendering |
| `exclusive_group` | string \| null | Group key for mutual exclusivity | Exclusive control rendering |
| `required` | boolean | Whether field must have a value | Validation |
| `min` / `max` | number \| null | Numeric bounds | Validation |
| `minLength` / `maxLength` | number \| null | String length bounds | Validation |
| `pattern` | string \| null | Regex pattern | Validation |

### Auth State

Represents the authentication status for a user session.

| Attribute | Type | Description | Used By |
|-----------|------|-------------|---------|
| `isAuthenticated` | boolean | Whether user has valid session | Shell, auth screen |
| `accessToken` | string | Bearer token (in-memory only) | API transport |
| `user` | object | User profile data | Shell header, session guard |

## Relationships

```
Catalog
    └── displays → Template[]
        └── Template.id → Sheet.template_id → opens → Sheet
        └── Sheet.id → URL → Sheet screen (F3 read-only)

AuthScreen
    └── validates → user credentials
    └── produces → AuthState
        └── AuthState.isAuthenticated → Shell (shows session state)

SheetScreen
    └── loads → Template (for schema)
    └── loads → Sheet (for data)
    └── renders → FieldSchema[]
        └── each field → F2 Field + F2 Control
```

## Data Flow (presentation only)

1. **Catalog** → loads `Template[]` → renders list with names/metadata → "Create Sheet" button → opens F2 dialog.
2. **Sheet** → receives `Sheet.id` (route param) → loads `Sheet` → loads `Template` → renders fields via F4 engine → footer shows dirty state + Save.
3. **Auth** → renders mode-specific form → validates client-side → posts credentials → receives `AuthState` → redirects to catalog.
4. **Your-sheet** → checks `SheetStore.hasSheet()` → shows empty message or redirects to sheet.
5. **Not-found** → `NOT_FOUND` response from Sheet open → shows message + link to catalog.
6. **Update-prompt** → service worker detects new version → shows message + update button.

## Token Mapping (presentation only)

Every visual value in the screens maps to a `--lg-*` token from F1:

| Screen Element | Token Usage | Example |
|----------------|-------------|---------|
| Page background | `--lg-color-bg` | `background: var(--lg-color-bg)` |
| Surface (cards, panels) | `--lg-color-surface` | `background: var(--lg-color-surface)` |
| Text | `--lg-color-text` | `color: var(--lg-color-text)` |
| Muted text | `--lg-color-text-muted` | `color: var(--lg-color-text-muted)` |
| Border | `--lg-color-border` | `border-color: var(--lg-color-border)` |
| Accent action | `--lg-color-accent` + `--lg-color-accent-text` | `background: var(--lg-color-accent); color: var(--lg-color-accent-text)` |
| Button size | `--lg-size-control`, `--lg-radius-sm/md` | `height: var(--lg-size-control); border-radius: var(--lg-radius-sm)` |
| Spacing | `--lg-space-*-` | `gap: var(--lg-space-16)` |
| Typography | `--lg-font-*`, `--lg-type-*` | `font-size: var(--lg-type-15); font-weight: var(--lg-font-weight-medium)` |
| Shadow | `--lg-shadow-1/2/3` | `box-shadow: var(--lg-shadow-2)` |

No literal values appear in the refactored screens.