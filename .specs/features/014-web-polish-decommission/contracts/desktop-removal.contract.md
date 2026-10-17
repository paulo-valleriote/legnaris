# Contract: Desktop Decommission (F10 / POL-07 / POL-08 / POL-09)

- Remove directory `desktop/`; file `ci-flutter.yml`; references in `Makefile`, `.gitignore`, root docs, `.specify/`.
- Update `.specify/memory/constitution.md` to reflect deprecated desktop (already v1.1.1 — verify no residual references to Flutter as active stack).
- Update root `AGENTS.md`; add `web/AGENTS.md` reference.
- Update `check_requirement_ids.py` to cover `*.ts` / `*.tsx`; verify with `make lint-specs`.
- No removal until POL-01 (parity) passes.
