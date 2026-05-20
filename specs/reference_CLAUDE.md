# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Overview

Alpheya is a wealth management platform monorepo containing 50+ services and applications. The platform provides portfolio management, trading, reporting, and AI-powered insights for wealth advisors and investors.

## Tech Stack Summary

| Language            | Services                                                                                                                | Framework                     | Build Tool      |
| ------------------- | ----------------------------------------------------------------------------------------------------------------------- | ----------------------------- | --------------- |
| **Go**              | hermes-api, facade, cerbos-policies, search-service, referral-service, sealed-secrets-ui, oauth2-proxy, patrimonio, cli | Connect RPC, gRPC             | Makefile        |
| **Python**          | ai-api, bank-ingestor, model-bank, bank-adapter                                                                         | FastAPI + Temporal            | Makefile + uv   |
| **TypeScript/Node** | frontend, admin-portal, odysseus, kindi, experience-api, nexus-adapter, azure-openai-proxy, alpheya-mcp                 | Next.js 14, React 18, Express | npm + Turborepo |

## Key Service Directories

- **`platform-services/`** - Main backend monorepo (Go + Node.js modules)
- **`frontend/`** - Turborepo frontend monorepo (advisor-desktop, reporting apps)
- **`alpheya-api/`** - Protocol Buffer definitions (API contracts)
- **`alpheya-common-packages/`** - Shared Go and TypeScript libraries
- **`ai-api/`** - LLM capabilities (FastAPI + Temporal)
- **`k8s-apps-gitops/`**, **`k8s-platform-gitops/`** - Kubernetes GitOps manifests

## Development Commands

### Platform Services (Go/Node.js Backend)

```bash
# Tool management (required first)
mise trust && mise install

# Build, test, lint
mise run build:all
mise run test:all
mise run lint:all
mise run fmt:all
mise run docker:build:all
```

### Frontend (Turborepo)

```bash
npm run dev                    # Run all apps
npm run dev:advisor-desktop    # Specific app
npm run build
npm run test
npm run lint                   # Biome linter
npm run check-types
```

### Python Services (ai-api, bank-ingestor)

```bash
make sync     # Install dependencies with uv
make tests    # Run pytest
make lint     # Ruff linting
make format   # Ruff formatting
```

### Go Services (hermes-api, facade, etc.)

```bash
make build
make test
make lint          # golangci-lint
make fmt           # gofumpt
```

### RULE

When creating PRs keep the description short and concise. 

### Running Single Tests

```bash
# Go
go test -v -run TestFunctionName ./path/to/package

# Python
uv run pytest path/to/test.py::test_function_name -v

# TypeScript (frontend)
cd apps/advisor-desktop && npm run test -- path/to/file.test.ts
```

## Architecture Overview

### Data Flow

```
External Banks/Systems → [File Ingestion] → Staging DB
                                    ↓
                         [Data Processor] (Temporal workflows)
                                    ↓
                         [Data Quality] → OLAP Database
                                    ↓
                         [Experience API] (GraphQL)
                                    ↓
                         [Frontend Apps]
```

### Domain Boundaries (from alpheya-api)

- **identity/** - Clients, Users, Teams
- **wealth/** - Accounts, Portfolios, Holdings, Transactions
- **order/** - Order creation, approval, execution
- **market/** - Assets, Prices, FX Rates, Corporate Actions
- **crm/** - Client relationships, Tasks, Notes
- **reference/** - Configuration (currencies, segments)

### Cross-Cutting Concerns

- **Auth**: OAuth2 Proxy + Cerbos (policy-based authorization)
- **Workflows**: Temporal for distributed processing
- **Observability**: OpenTelemetry (traces, metrics, logs)
- **Database**: PostgreSQL + TimescaleDB, migrations via DBMate
- **API Contracts**: Protocol Buffers with Buf

## Code Conventions

### Go

```go
// Error handling - always wrap errors
if err != nil {
    return fmt.Errorf("failed to process data: %w", err)
}

// Structured logging
logger.Info("processing request", "request_id", reqID, "duration", duration)
```

- Use SQLC for type-safe database access
- Colocate unit tests in `*_test.go` in same directory
- Shared code goes in `modules/go-common` only if used by ≥2 modules

### TypeScript

- Use `import type { … }` for type-only imports
- Prefer `type` over `interface` unless merging is needed
- Components use kebab-case filenames (`my-component.tsx`)
- Favor React Server Components; minimize `'use client'`

### Python

- FastAPI for REST APIs
- Pydantic for data validation
- pytest for testing with coverage

## Testing Guidelines

1. **TDD**: scaffold stub → write failing test → implement
2. **Prefer integration tests** over heavy mocking
3. **Separate** pure-logic unit tests from DB-touching integration tests
4. **Test assertions**: prefer `assert.Equal(t, expected, actual)` over weaker assertions
5. **No trivial tests**: every test must be able to fail for a real defect

## Testing Conventions

Always use proper assertion patterns in TypeScript tests (`expect(...).toThrow`, `expect(...).rejects`) — never use try/catch with non-null assertions. Prefer `toThrow`/`toMatchObject` over string-matching for error validation.

## CI/CD & Quality Gates

```bash
# Must pass before merge
mise run lint:all    # or npm run lint for frontend
mise run test:all    # or npm run test
golangci-lint run    # Go services
govulncheck ./...    # Security scanning
```

## Git Conventions

- **Conventional Commits**: `feat:`, `fix:`, `chore:`, `refactor:`, etc.
- Do not reference Claude/Anthropic/Cursor in commit messages
- PR labels: `ready for review` triggers Slack notifications

## Module-Specific Notes

### Platform Services Modules (`/platform-services/modules/`)

- **file-ingestion**: Ingests files from Azure Blob/S3
- **data-processor**: Temporal workflows for entity processing
- **experience-api**: GraphQL aggregation layer (Drizzle ORM)
- **integration**: External-facing third-party APIs (handle idempotency)
- **migrate**: Centralized database migrations

### Frontend (`/frontend/`)

See `/frontend/CLAUDE.md` for detailed frontend guidance including:
- App structure (advisor-desktop, reporting, dq)
- GraphQL codegen workflow
- Storybook and E2E testing

### Useful Shortcuts (Platform Services)

When working in `/platform-services/`, users may invoke these shortcuts:
- **QNEW** - Review best practices before starting
- **QPLAN** - Verify plan consistency with codebase
- **QCODE** - Implement with tests, formatting, linting
- **QCHECK** - Review changes against all checklists
- **QGIT** - Commit with Conventional Commits format
