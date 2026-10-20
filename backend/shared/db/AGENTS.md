# AGENTS.md — backend/shared/db (MongoDB Initialization)

## Responsibility

- MongoDB client initialization and connection management
- Provides `*mongo.Database` to repositories

## Conventions

- Declare own typed errors — **do NOT import `apperr`**
- Wrap errors with `fmt.Errorf("...: %w", err)`
- Service layer translates to `apperr` at boundary
- All identifiers, comments, logs in English