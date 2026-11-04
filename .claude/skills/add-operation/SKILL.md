---
name: add-operation
description: Build an operation of the client's contract (api/openapi.json) so it answers instead of 501 — the entity, migration, repo, use case, handler and tests, with permission, branch, masks and audit coming from the contract. Use when asked to implement an endpoint, an operationId, a module of the client, or "make X work against the real server".
---

# Build an operation of the client's contract

The contract is the client's (`api/openapi.json`, copied by `make contract-sync FROM=<client repo>`):
never edit it here. If it is wrong or missing something, say so, and the client changes it first.
Every operation is already routed and answers 501 until it has a handler; the guard
(`internal/controller/restapi/clientapi/guard.go`) already applies `x-permission`,
`x-branch-scoped` for the asked workspace, the body's schema, `x-masks` and `x-audit`. You write
what the contract can't say: the data, the record-level rules, the shape of the answer.

`patients` is the worked example: `patList`, `patGet`, `patCreate`, `patUpdate`. Read it first.

## 1. Read the operation

```sh
go run ./cmd/contract -all | grep <tag or path>
python3 -c "import json;d=json.load(open('api/openapi.json'));print(json.dumps(d['paths']['/x'],indent=1))"
```

Note its `operationId`, parameters, request body and `200` schema (follow each `$ref` into
`components.schemas`), `x-permission`, `x-branch-scoped`, `x-masks`, `x-audit`. Each `x-audit` line
is `<action> <resource>[: rule]`: the guard writes it, once a window for `per N minutes`; a line
`for one who …` is yours to write in the handler, only when it holds.

## 2. Inside out, as CLAUDE.md › Adding a whole domain says

1. **Entity** (`internal/entity/<domain>.go`): the record and its query/patch types. Refusals are
   `ErrNotFound`, `ErrForbidden`, `ErrNotMember`, `entity.Invalid(field, "words for people")`.
2. **Migration** (`make migrate-create create_<table>`), up and down. Money is an integer of the
   smallest unit; a record of a branch has its `workspace`; codes come from a sequence.
3. **Repo** (`internal/repo/contracts.go`, `internal/repo/persistent/<domain>/`, its `tracing.go`):
   Squirrel only. A list answers its total and summary over **every match**, in SQL, not from the
   page. A day is `(at AT TIME ZONE 'Asia/Ho_Chi_Minh')::date`. Spans carry ids, never a patient's
   name, phone or what was searched.
4. **Use case** (`internal/usecase/contracts.go`, `internal/usecase/<domain>/`, its `tracing.go`):
   every method takes `*entity.Principal`. Here go the rules the guard can't see: a record of a
   branch not theirs (`p.Works(ws)`: `"true"` → `ErrNotMember`, `"hidden"` → `ErrNotFound`,
   `"filter"`/`"clinical"` → narrow the query), a field one may change only with a grant, what a
   masked reader may search by. Then `make mock`, and `internal/usecase/<domain>_test.go`.
5. **Handler** (`internal/controller/restapi/clientapi/<domain>.go`): decode the body (the guard
   has already held it to its schema), call the use case with `c.Context()`, encode the contract's
   shape (a struct with its JSON names; required fields always present, `[]` not `null`), refuse
   with `a.refuse(c, err)`. An update leaves `c.Locals(_localChanges, changes)` for the log.
   Register it by its `operationId` in `NewRoutes`' `built` map; one the contract lacks fails at start.
6. **Wiring** (`internal/app/app.go`): the repo and use case in `initUseCases`, the use case in
   `clientapi.Deps`.

## 3. Prove it

- `clientapi/clientapi_test.go`: a fake of the use case, and a test per behaviour that matters,
  each answer checked with `h.answers(t, "<operationId>", r)` (the contract's own `200` schema)
  and each refusal with `h.refusal(t, r, status)` (the contract's `Error`): masks for a reader
  without the grant, the branch refused, the audit line written.
- `internal/usecase/<domain>_test.go`: the record-level rules, against gomock mocks.
- `integration-test/<domain>_test.go` or `client_test.go`: the same over Postgres, seeding with
  `seed(t)` and calling with `signIn(t, email)`; each answer through `answers(t, opID, r)`.

## 4. Done

```sh
make format && make linter-golangci && make test
make contract-status        # one fewer answering 501
make verify                 # before the push: CI's jobs here, on an empty database (Docker)
```

In the PR: the operationIds built, what you decided the contract left open (under Assumptions),
and each rule you couldn't build (a `for one who` line, a module this one reads) under Not done.
