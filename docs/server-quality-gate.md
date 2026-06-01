# Server Quality Gate

This server cleanup is refactor-only. Do not rename HTTP routes, request JSON,
response JSON, exported Go interfaces, or operator-facing behavior unless a
separate feature plan explicitly calls for it.

## Required Checks

Run these checks before and after each meaningful refactor:

```bash
go test ./...
go vet ./...
go test -cover ./...
```

Coverage is a regression watchpoint, not a vanity metric. Keep these packages at
or above the current baseline unless a deliberate test tradeoff is documented:

- `internal/app`: about 80.9%
- `internal/web`: about 63.1%

## Optional Database Checks

Postgres-backed tests are skipped unless `TEST_DATABASE_URL` is set. When a local
test database is available, run:

```bash
TEST_DATABASE_URL="postgres://atol:atol@localhost:5432/atol_test?sslmode=disable" go test ./internal/settings ./internal/receiptsnapshot ./internal/imageeditor
```

These tests cover store behavior that ordinary unit tests cannot fully exercise.

## Static Web Client Debt

`internal/web/static/app.js` is intentionally out of the first Go refactor pass.
It is a high-risk file because it mixes bootstrap state, settings forms,
scheduler UI, receipt preview rendering, QR preview code, image editing, and
print actions in one browser-global script.

When the static client is touched, split it in a separate pass by behavior:

- bootstrap and API helpers
- settings forms
- scheduler UI
- receipt preview and QR preview
- image editor
- text print editor

Keep the server API stable while doing that split.
