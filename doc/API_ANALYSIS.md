# API Analysis

## Scope

This document reviews the examples under `doc/examples/*.md` from the perspective of the developer who consumes the API, and proposes a new `set` operation with upsert semantics.

The goal is not to redesign the whole API at once, but to identify the main friction points and define an incremental path that improves usability without fighting the current implementation model.

## Main Findings From the Current Examples

### 1. The API contract is not predictable enough

The examples expose several styles at the same time:

- `GET /v1/collections`
- `POST /v1/collections`
- `POST /v1/collections/{collection}:find`
- `POST /v1/collections/{collection}:insert`
- `POST /v1/collections/{collection}:remove`
- `POST /v1/collections/{collection}:setDefaults`

For a developer integrating the service, this means there is no obvious rule for when an operation is a resource-oriented route and when it is an RPC-style action.

This is not only a stylistic problem. It also makes client generation, onboarding, caching, and HTTP expectations harder than necessary.

### 2. Response formats are inconsistent

The examples mix:

- a JSON object for a single inserted or patched document
- a JSON array for collection and index listing
- newline-delimited JSON for `find`, `remove`, and multi-item `insert`

NDJSON can be a good fit for streaming, but the current examples do not make that explicit enough. From the client perspective, it is important to know upfront whether the response must be decoded as:

- one JSON document
- one JSON array
- a stream of JSON documents separated by newlines

Right now that has to be inferred from examples instead of being part of the contract.

### 3. Important defaults are implicit

Some defaults are useful internally but surprising for users:

- `find` defaults to `limit = 1` when the request omits it
- `insert` creates the collection automatically if it does not exist
- `setDefaults` also creates the collection automatically
- collection defaults inject `"id": "uuid()"` unless overwritten

These behaviors are powerful, but they are not obvious from the examples. Hidden defaults increase the chance of subtle bugs in client code.

### 4. Error semantics are weak for client developers

Several error cases in the examples return `500 Internal Server Error` for situations that are not server failures:

- collection not found
- index not found

From the consumer point of view, this is a major issue. A `404` or `400` gives a clear remediation path. A `500` suggests retry or operator escalation.

The current error body is also too generic:

```json
{
  "error": {
    "description": "Unexpected error",
    "message": "collection not found"
  }
}
```

It is readable, but not very machine-friendly.

### 5. There are naming inconsistencies and example errors

The examples reveal terminology drift:

- the file name says `delete`, but the route uses `:remove`
- `remove_-_by_btree_with_filter.md` actually calls `:find` and shows a `find` response
- `drop_collection.md` shows `200 OK`, while the actual acceptance flow expects `204 No Content`
- `list_indexes.md` uses `POST ...:listIndexes` instead of a more natural `GET`

For a developer reading the examples as the de facto spec, this creates distrust in the contract.

### 6. The reference docs and the examples are out of sync

The files under `doc/book/src/api_reference/*.md` describe simpler contracts that do not match the examples or current handlers.

Examples:

- `find.md` documents query parameters over HTTP `GET`-like semantics, while the examples and implementation use `POST ...:find` with JSON input
- `patch.md` documents patching by `id`, while the implementation supports traversal by filter or index
- `insert.md` describes an array of items, while the implementation accepts one JSON document or NDJSON

This mismatch increases integration risk more than missing documentation would.

### 7. The API is optimized for internal flexibility more than consumer clarity

The current model is operationally convenient:

- generic traversal over collection or index
- merge patch over raw JSON payloads
- streaming writes and reads

But the consumer still needs a stable mental model:

- how to read one document
- how to insert many
- how to update many
- how to update one deterministically
- how to perform idempotent write operations

At the moment, the API exposes the building blocks more clearly than the workflows.

## Improvements That Would Help API Consumers

### 1. Make response shape explicit in every operation

Every operation should state one of these response modes:

- `application/json`: one JSON object
- `application/json`: one JSON array
- `application/x-ndjson`: stream of JSON objects

This should appear in both examples and reference docs.

A practical rule would be:

- single-resource operations return one JSON object
- list/search/remove-many operations return `application/x-ndjson`
- metadata lists may return one JSON array

### 2. Document the hidden behaviors as first-class features

The examples should explicitly call out:

- automatic collection creation on `insert`
- default `id` generation
- `find` default limit
- how `null` behaves in `setDefaults`

If a behavior is important enough to affect data shape or persistence, it should not be implicit.

### 3. Normalize error responses

A better error body for client code would be:

```json
{
  "error": {
    "code": "collection_not_found",
    "message": "collection 'customers' not found"
  }
}
```

Suggested mappings:

- `400 Bad Request`: invalid JSON, invalid parameters, invalid index payload
- `404 Not Found`: collection or document not found
- `409 Conflict`: unique index conflict
- `422 Unprocessable Entity`: valid JSON with invalid write semantics

This helps developers branch correctly without parsing free-form strings.

### 4. Add examples around deterministic single-document workflows

The examples currently emphasize traversal and bulk operations, but most application developers first need:

- get document by id
- insert document
- patch document by id
- set or upsert document by id

The recently added `GET /v1/collections/{collectionName}/documents/{documentId}` endpoint should be documented with examples. It is much easier to consume than a generic `find` in common use cases.

### 5. Clarify what is streamed and why

NDJSON is a valid design choice, especially for large results, but it should be described as a capability instead of an accident of implementation.

Recommended additions:

- explain that `find`, bulk `insert`, and bulk `remove` can stream multiple JSON documents
- show one example of line-by-line decoding in Go, JavaScript, and shell
- specify whether partial success is possible when streaming writes

### 6. Reduce terminology drift

The docs should consistently choose one term per concept:

- `remove` or `delete`, but not both
- `setDefaults` or `defaults`, but not mixed descriptions
- `document`, `item`, or `row`, but with one public term

Internally there may still be rows and commands, but the public API should keep a smaller vocabulary.

### 7. Distinguish three write intents clearly

From the client point of view, write operations are easier to understand when they map to intent:

- `insert`: create only, fail if the unique key already exists
- `patch`: update only, fail or no-op if nothing matches
- `set`: update if found, insert if not found

This is the missing piece in the current API.

## Proposed `set` Operation With Upsert Semantics

### Why `set` is needed

A client often wants an idempotent write:

- "Set these fields for document `id = user-42`"
- "If `user-42` does not exist, create it"

Today the client has to:

1. `find` first
2. branch on the result
3. call `patch` or `insert`
4. handle race conditions and unique conflicts

That is more round trips and pushes write coordination to the client.

### Recommended contract

The operation should be deterministic and should only allow lookup strategies that can match at most one document.

Recommended endpoint:

```http
POST /v1/collections/{collectionName}:set
```

Recommended request body:

```json
{
  "match": {
    "id": "user-42"
  },
  "set": {
    "name": "Fulanez",
    "verified": true,
    "country": "ES"
  }
}
```

Semantics:

- if a document with `id = "user-42"` exists, apply merge patch with `set`
- if it does not exist, insert a new document built from `match + set`
- defaults still apply during insert for fields that remain absent

Resulting inserted document:

```json
{
  "id": "user-42",
  "name": "Fulanez",
  "verified": true,
  "country": "ES"
}
```

Resulting updated document:

```json
{
  "id": "user-42",
  "name": "Fulanez",
  "verified": true,
  "country": "ES"
}
```

This makes `set` a natural companion to `insert` and `patch`.

### Why `match.id` should be the first version

The codebase already has a dedicated path for document lookup by id. Starting with `id` keeps the first implementation simple and gives the API a high-value workflow immediately.

It also avoids ambiguous upserts such as:

- `filter: {"country": "ES"}`
- `index: "by-name", value: "john"`

Those queries can match zero, one, or many rows, which is incompatible with deterministic upsert semantics.

### Suggested future extension

Once the `id`-based version is stable, the contract can be expanded to support lookup by unique index.

Example:

```json
{
  "match": {
    "index": "email",
    "value": "ops@example.com"
  },
  "set": {
    "name": "Ops",
    "role": "admin"
  },
  "insert": {
    "email": "ops@example.com"
  }
}
```

Semantics:

- lookup uses a unique index
- update applies `set`
- insert uses `insert + set`

The extra `insert` object is useful because index lookup metadata is not always enough to reconstruct the full document to insert, especially for compound indexes.

### Response shape

`set` should return one JSON object and a stable operation marker:

```json
{
  "operation": "inserted",
  "document": {
    "id": "user-42",
    "name": "Fulanez",
    "verified": true,
    "country": "ES"
  }
}
```

or:

```json
{
  "operation": "updated",
  "document": {
    "id": "user-42",
    "name": "Fulanez",
    "verified": true,
    "country": "ES"
  }
}
```

This is better for clients than inferring the outcome from status code alone.

Suggested status codes:

- `201 Created` when inserted
- `200 OK` when updated

### Error behavior

Recommended error cases:

- `400 Bad Request`: missing `match.id`, invalid JSON, unsupported match mode
- `409 Conflict`: insert path hits a unique index conflict
- `422 Unprocessable Entity`: `set` payload is not an object when object semantics are required

If future versions support `match.index`, then:

- `404 Not Found` should not happen for a missing matched document during `set`; it should insert instead
- `409 Conflict` should happen if the selected lookup index is not unique and the contract requires deterministic upsert

## Suggested Implementation Strategy

### Phase 1: support `match.id` only

This is the safest implementation path with the current code:

1. Add a new handler `api/apicollectionv1/set.go`
2. Register `box.ActionPost(set)` in `api/apicollectionv1/0_build.go`
3. Decode:

```json
{
  "match": { "id": "..." },
  "set": { ... }
}
```

4. Reuse `findRowByID(...)`
5. If row exists:
   apply `col.Patch(row, set)`
6. If row does not exist:
   build `newDocument := merge(match, set)`
   call `col.Insert(newDocument)`
7. Return `{ "operation": "...", "document": ... }`

This version already solves the most common upsert use case.

### Phase 2: support unique index lookup

After Phase 1, `match` can be extended to:

```json
{
  "match": {
    "index": "my-unique-index",
    "value": "..."
  },
  "set": { ... },
  "insert": { ... }
}
```

Implementation constraints:

- the index must exist
- it must be unique
- the lookup must resolve to at most one row
- if not found, `insert` must contain the fields required to satisfy that index

### Important internal caveat

Current patching updates the row payload before re-inserting the row into indexes. If index insertion fails, the code comments already note that rollback is incomplete.

That matters for `set`, because an upsert endpoint will likely become a primary write path. Before promoting `set` heavily in the docs, index-safe patch rollback should be fixed so that a failed update cannot leave the row payload and indexes temporarily inconsistent.

### Concurrency expectations

For `match.id` upsert, two concurrent writers targeting the same id may race:

- both can observe "not found"
- both can try to insert
- one should succeed
- the other should get `409 Conflict`

That is acceptable for a first version, as long as it is documented.

If stronger semantics are desired later, the implementation can add a collection-level upsert lock keyed by normalized document id.

## Concrete Documentation Changes Recommended

Short term:

- add a new example for `GET /v1/collections/{collection}/documents/{id}`
- fix broken examples and naming inconsistencies
- document NDJSON explicitly
- document hidden defaults explicitly
- align `doc/book/src/api_reference/*.md` with the actual handlers
- add `set` as the canonical upsert operation

Medium term:

- standardize error codes and status codes
- decide which operations are resource-oriented and which are RPC-style
- consider a cleaner single-document surface around `/documents/{id}`

## Recommended First Public Example For `set`

```sh
curl -X POST "https://example.com/v1/collections/users:set" \
-d '{
    "match": {
        "id": "user-42"
    },
    "set": {
        "name": "Fulanez",
        "verified": true,
        "country": "ES"
    }
}'
```

Insert response:

```http
HTTP/1.1 201 Created
Content-Type: application/json

{
    "operation": "inserted",
    "document": {
        "id": "user-42",
        "name": "Fulanez",
        "verified": true,
        "country": "ES"
    }
}
```

Update response:

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
    "operation": "updated",
    "document": {
        "id": "user-42",
        "name": "Fulanez",
        "verified": true,
        "country": "ES"
    }
}
```

## Conclusion

The existing examples show that the storage engine is already close to supporting a developer-friendly API, but the public contract still exposes too many internal details and inconsistencies.

The most valuable addition is a deterministic `set` operation with upsert semantics, starting with `match.id`. It reduces round trips, makes client code simpler, and gives the API a clear answer to one of the most common persistence workflows: update-or-insert.
