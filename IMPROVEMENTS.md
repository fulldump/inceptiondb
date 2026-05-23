# InceptionDB Improvements

This document reviews InceptionDB as a future commercial and open source database product. It focuses on product readiness, performance stability, and internal architecture.

## Current Position

InceptionDB is a durable in-memory JSON document database with an append-only WAL, HTTP API, embedded Go usage, collection-level indexes, and very fast in-memory record structures.

The project already has strong technical foundations for a small operational database:

| Area | Current state |
| --- | --- |
| Core model | In-memory documents with append-only journal recovery |
| API | HTTP v1 endpoints for collections, insert, find, patch, remove, indexes and size |
| Storage | Binary WAL with operation code, id, length and CRC32 |
| Indexes | Map, BTree, FTS and PK-style indexes |
| Performance | Excellent in-memory throughput in record benchmarks |
| Tests | Unit and acceptance tests pass with `make test` |
| Packaging | CLI, Dockerfile, static UI and release Make targets exist |

The main gap is not raw speed. The main gap is product hardening: predictable durability semantics, operational observability, API polish, benchmark coverage that matches real workloads, and internal simplification where highly optimized code is now mixed with product-facing behavior.

## Verification Run

Commands executed on 2026-05-23:

```sh
make test
go test -bench=. -benchmem ./collection/records
go test -bench=. -benchmem ./collection ./collection/stores ./collection/fson ./simdscan
```

Results:

| Command | Result |
| --- | --- |
| `make test` | Pass |
| `./collection/records` benchmarks | Pass |
| `./collection`, `./collection/stores`, `./collection/fson`, `./simdscan` benchmarks | Pass |

Important benchmark observations from this run:

| Component | Observation |
| --- | --- |
| `RecordsUltra` | Mixed stress around 19.7M to 22.0M ops/s across 16 to 128 workers; insert around 17.4M to 22.6M ops/s; set around 21M ops/s and very stable. |
| `RecordsHyper` | Insert around 17.9M to 19.0M ops/s; set around 27.5M to 28.0M ops/s; mixed stress has more variance, around 18.1M to 26.8M ops/s. |
| `RecordsTurbo` | Mixed stress around 17.8M to 20.4M ops/s; set around 25.9M to 26.9M ops/s. |
| `RecordsFast` | Very fast mixed stress around 29.6M to 30.6M ops/s, but insert/set are lower than specialized implementations. |
| `Patch` | Initial raw/compiled patch work reduced the benchmark to around 399 ns/op, 504 B/op and 5 allocations/op. More work remains for index-aware deltas and raw JSON rewrite. |
| Store compression | Snappy wrapper measured around 183 ns/op and 224 B/op; no compression around 51 ns/op and 0 B/op. |
| JSON field scan | `simdscan.GetField` around 33 ns/op versus `jsonparser.Get` around 45 ns/op and stdlib unmarshal around 1928 ns/op. |

Additional end-to-end benchmark results from `cmd/bench` show that the runtime path is also very strong, not only the isolated primitives:

| Workload | Throughput |
| --- | --- |
| `INSERT` | 8,785,855 rows/sec |
| `INSERT` recovery/open | 8,780,488 rows/sec |
| `INSERTPK` | 5,890,949 rows/sec |
| `INSERTPK` recovery/open | 2,374,067 rows/sec |
| `INSERTBTREE` | 1,766,108 rows/sec |
| `INSERTBTREE` recovery/open | 3,165,092 rows/sec |
| `RETRIEVEBTREE` | 5,412,336 docs/sec |
| `RETRIEVEFS` insert | 9,249,223 rows/sec |
| `RETRIEVEFS` full scan, no filter | 13,145,243 docs/sec |
| `RETRIEVEFS` full scan with filter | 4,265,889 docs/sec |
| `PATCH` | 2,735,629 rows/sec |
| `PATCH` recovery/open | 9,973,489 rows/sec |
| `REMOVE` | 1,622,765 rows/sec |
| `REMOVE` recovery/open | 14,321,484 rows/sec |

These numbers are excellent for an in-memory JSON database with an HTTP API, but they should be treated as internal release benchmarks until the methodology is fixed: dataset shape, document size, durability mode, hardware, worker count, response mode, index cardinality and variance must be recorded with every run.

## Product Improvements

### P0: Define Product Contract

The README describes InceptionDB as durable and strongly consistent at document level, but the actual product contract needs sharper wording before selling or releasing broadly.

Recommended improvements:

| Topic | Recommendation |
| --- | --- |
| Durability modes | Document exactly what `wait=true` guarantees and what default async writes can lose during process crash, OS crash or power loss. |
| Consistency | Define atomicity for insert, patch, delete, index updates and multi-document operations. |
| Limits | Publish supported document size, collection size, number of collections, index cardinality and memory expectations. |
| Compatibility | Version the HTTP API and WAL format explicitly. |
| Failure behavior | Document what happens on corrupted/truncated WAL records and how operators recover. |

Why this matters: buyers need a clear SLA-like contract more than peak benchmark numbers.

### P0: Add Authentication and Authorization

The public docs mention authentication/authorization material, but the runtime API code does not show an auth layer. A database product cannot expose write/delete/index operations unauthenticated by default.

Recommended minimum:

| Capability | Recommendation |
| --- | --- |
| Authentication | Add API key or bearer token support first; keep it simple and dependency-light. |
| Authorization | Start with read/write/admin scopes. |
| Defaults | Bind to localhost by default, but require explicit auth configuration for non-local deployments. |
| Audit | Log admin operations such as create/drop collection, create/drop index and defaults changes. |

### P0: Productize Operations

Recommended operational endpoints and behavior:

| Area | Recommendation |
| --- | --- |
| Health | Add `/healthz` for process liveness and `/readyz` for database readiness after recovery. |
| Metrics | Expose Prometheus-compatible counters/histograms for requests, WAL appends, fsync duration, recovery duration, index lag and memory. |
| Backups | Provide documented safe backup procedure for WAL files and snapshots. |
| Logging | Replace `fmt.Println`/`fmt.Printf` with structured logging and levels. |
| Shutdown | Ensure HTTP shutdown waits for database close and returns errors visibly. |

### P1: Documentation for Buyers and Operators

The current README is useful but still project-oriented. It should become a product entry point.

Recommended documents:

| Document | Purpose |
| --- | --- |
| `README.md` | Product pitch, quick start, supported use cases, non-goals and safety notes. |
| `docs/operations.md` | Running in production, durability modes, backup/restore, monitoring and upgrades. |
| `docs/api.md` | Stable API examples with status codes and error shapes. |
| `docs/performance.md` | Reproducible benchmark methodology, hardware, dataset, results and interpretation. |
| `docs/architecture.md` | WAL, recovery, records, indexes, consistency and concurrency model. |

### P1: Packaging and Release Hygiene

Current packaging exists, but versions are inconsistent:

| File | Issue |
| --- | --- |
| `go.mod` | Uses `go 1.26.1`. |
| `.github/workflows/go.yml` | Uses Go `1.21.2`. |
| `Dockerfile` | Uses `golang:1.24.1`. |

Recommended improvements:

| Area | Recommendation |
| --- | --- |
| Go version | Use one supported Go version across `go.mod`, CI and Dockerfile. |
| Docker image | Add labels, non-root runtime user if not using scratch, documented volume path and healthcheck. |
| Releases | Publish checksums, SBOM and signed artifacts. |
| Config | Document environment variables generated by `goconfig`; avoid hidden config names. |
| License | Keep license visible and add contribution guidelines before open sourcing. |

## Performance Improvements

### P0: Standardize End-to-End Benchmarks

The repository already has end-to-end benchmarks in `cmd/bench`. They start the service automatically, exercise the HTTP API and clean up temporary data. The next step is to standardize them so results are reproducible and comparable between releases.

Keep and formalize coverage for:

| Workload | Metrics |
| --- | --- |
| HTTP insert single document | throughput, p50/p95/p99 latency, error rate, allocations |
| HTTP streaming insert | throughput, backpressure behavior, response streaming cost |
| Insert with `wait=false` and `wait=true` | fsync cost and durability/latency tradeoff |
| Insert with no index, PK index, unique map index, non-unique BTree index and FTS index | index maintenance overhead |
| Find by index | p50/p95/p99 latency and documents/sec returned |
| Full scan with filter | scan throughput and memory usage |
| Patch | latency, allocations and index update overhead |
| Delete/remove by index | throughput and fragmentation/tombstone behavior |
| Recovery | startup time per WAL size and per operation count |

Use fixed datasets and publish hardware details so numbers can be compared between releases. Also make `--test all` execute all scenarios, because the switch currently accepts `ALL` but does not run the suite.

### P0: Define Performance Budgets

Create release gates for target workloads. Example initial budgets:

| Operation | Suggested budget |
| --- | --- |
| In-memory insert without fsync | No regression above 10 percent versus previous release. |
| Insert with PK index | No regression above 10 percent versus previous release. |
| Patch | Allocation count must trend down further; current result is around 5 allocs/op after the first compiled-patch optimization. |
| Recovery | Maximum startup time per million WAL operations should be measured and bounded. |
| Indexed find | p99 latency should be tracked under concurrent writes. |

The exact budgets should be chosen from product SLOs, not from isolated microbenchmarks.

### P1: Make Performance Homogeneous Across Operations

Current performance is excellent for record structures, but less homogeneous across the database surface.

Recommended focus areas:

| Area | Reason |
| --- | --- |
| Patch allocations | `Patch` is allocation-heavy compared with insert/set primitives. Reduce JSON conversion/marshal work where possible. |
| Traversal variance | `RecordsUltra` and `RecordsHyper` have slower traversals than `RecordsFast`/`RecordsTurbo`; choose the default based on whole-product workload, not only insert/set. |
| Async index queue | `idxReqs` has a fixed capacity of 1,000,000 and non-unique indexes update asynchronously. Add metrics, backpressure and documented consistency semantics. |
| StoreAsync error handling | The async worker ignores append errors inside the batch loop and returns only flush/sync errors. Preserve first append error and report it to waiting callers. |
| WAL compaction | Deletes and updates keep growing the WAL. Implement snapshots or compaction to control recovery time and disk usage. |

### P1: Add Soak and Race Testing

Recommended commands for CI or nightly jobs:

```sh
go test -race ./...
go test -run Test -count=100 ./collection ./database ./api/...
go test -bench=. -benchmem -count=10 ./collection/records
```

Add long-running soak tests with concurrent insert/find/patch/delete and periodic process restarts to validate WAL recovery under realistic load.

## Architecture Improvements

### P0: Clarify the Consistency Model Around Async Indexes

`Collection.Insert`, `Delete`, `Patch` and `Update` synchronously update unique indexes but enqueue non-unique index updates asynchronously. That can make non-unique indexed reads lag behind records.

Recommended options:

| Option | Recommendation |
| --- | --- |
| Product contract | If this is intentional, document indexed reads as eventually consistent for non-unique indexes. |
| Strict mode | Add a collection or request-level option to force all index updates to complete before acknowledging writes. |
| Metrics | Track index queue depth, lag and dropped/blocked writes. |
| Tests | Add tests that assert expected behavior before and after `SyncIndexes`. |

### P0: Fix Transactional Rollback Gaps

Several write paths update memory/indexes before WAL persistence or do partial rollback.

Examples:

| Path | Risk |
| --- | --- |
| `Delete` | Removes unique index before WAL append; if append fails, the record remains but the index may be inconsistent. |
| `Patch`/`Update` | Replaces record before validating final index insertion and WAL append; rollback is noted as incomplete. |
| `CreateIndex` | Adds index to the collection before fully building and persisting it, then attempts rollback on error. |

Recommended improvement: centralize writes in a small transaction-like sequence with explicit prepare, apply, persist and rollback steps. Keep it simple, but make all failure cases deterministic.

### P0: Protect Shared Database State

`database.Database.Collections` is a plain map accessed by API/service paths and lifecycle operations. There is an existing TODO noting `DropCollection` is not thread-safe.

Recommended improvement:

| Change | Reason |
| --- | --- |
| Add `sync.RWMutex` around collection map access | Avoid concurrent map writes/panics under create/drop/list/get. |
| Do not expose the raw map | `Service.ListCollections` currently returns the map directly. Return a copy. |
| Remove duplicate ownership | `Service` stores `collections: db.Collections`; use `Database` methods instead. |

### P1: Separate Public Product Code From Experimental Internals

The code contains multiple record implementations (`Correct`, `Fast`, `Turbo`, `Ultra`, `Hyper`), store wrappers (`async`, `flusher`, `crazy`, `snappy`), and experimental parser packages. This is useful for development, but confusing as a product architecture.

Recommended structure:

| Package | Purpose |
| --- | --- |
| `database` | Lifecycle, collection registry, config, recovery orchestration. |
| `collection` | Stable document operations and index coordination. |
| `wal` or `storage/wal` | Append/replay/compact/snapshot logic. |
| `index` | Map, BTree, FTS, PK implementations and shared contracts. |
| `records` | Internal record engine with one production default and benchmark-only alternatives clearly marked. |
| `internal/bench` or `cmd/bench` | Experimental workloads and tools. |

### P1: Naming and API Cleanup

Recommended renames and cleanup:

| Current | Recommended |
| --- | --- |
| `Servicer` | `Service` interface or `CollectionService`; `Servicer` is uncommon and already has a TODO. |
| `IndexBtree` | `IndexBTree` for consistent acronym capitalization. |
| `rawstore_name`, `wrapstore_name`, `records_name` | `rawStoreName`, `wrapStoreName`, `recordsName`. |
| `StoreDisk` | `WALStore` or `DiskWAL` if it is specifically the journal. |
| `DeleteCollection`/`DropCollection` | Pick one term; databases usually use `DropCollection`. |
| `collection.Index` | `CreateIndex`; current alias adds little value. |
| Spanish comments/errors | Convert product code comments and errors to English for open source consistency. |

### P1: Improve Error Model and HTTP Status Codes

Acceptance tests currently include TODOs where 500 should be 400 or 404. API handlers often return raw errors with TODOs for wrapping.

Recommended improvement:

| Error type | HTTP status |
| --- | --- |
| Bad JSON/request shape | 400 |
| Collection/index/document not found | 404 |
| Unique/index conflict | 409 |
| Database loading | 503 |
| Internal invariant or storage failure | 500 |

Use one error response shape everywhere with stable fields such as `code`, `message`, `details` and `request_id`.

### P1: WAL Evolution and Recovery

The WAL is compact and has CRC validation, which is good. Productizing it needs lifecycle work.

Recommended improvements:

| Area | Recommendation |
| --- | --- |
| Format version | Add a WAL header with magic bytes and version. |
| Truncated tail | Treat cleanly detectable partial tail records according to a documented recovery policy. |
| Snapshots | Periodically write compact snapshots to avoid replaying full history forever. |
| Compaction | Remove superseded updates and deletes from active replay path. |
| Recovery reporting | Return structured stats: records replayed, bytes read, corrupt records, duration. |

### P2: Index Semantics and Query Planning

The README says users must explicitly select an index. That keeps the engine simple, but product users will expect guardrails.

Recommended improvements:

| Area | Recommendation |
| --- | --- |
| Query validation | Reject requests that reference missing indexes or malformed bounds with 400/404, not raw internal errors. |
| Index selection | Keep explicit index selection initially, but add documentation and UI hints. |
| BTree types | Define ordering between strings, numbers, booleans and nulls as part of the API contract. |
| FTS | Mark current FTS as basic token matching unless ranking, stemming and language behavior are added. |

### P0: Reorganize Architecture Around Explicit Layers

The current code works, but several packages mix product-facing behavior with low-level implementation details. `collection.Collection` coordinates records, WAL, defaults, indexes, recovery and JSON mutation. `service.Service` owns a direct pointer to `database.Database` and also caches `db.Collections`. `bootstrap.Bootstrap` constructs runtime dependencies directly. This makes the system fast, but harder to evolve safely.

Recommended target layers:

| Layer | Responsibility | Should not know about |
| --- | --- | --- |
| `cmd/inceptiondb` | CLI/config loading and process entrypoint | WAL internals, records internals |
| `bootstrap` or `app` | Dependency graph assembly and lifecycle wiring | HTTP route implementation details |
| `transport/http` | HTTP request/response decoding, status codes and streaming | Records, WAL files, index internals |
| `service` | Use cases: create collection, insert, find, patch, remove, index management | Concrete stores/records implementations |
| `database` | Collection registry, lifecycle, open/close, catalog persistence | HTTP, request bodies |
| `collection` | Document operations, write pipeline, index coordination, defaults | Disk filenames, CLI config |
| `storage/wal` | Append, replay, sync, compaction, snapshots | Collection semantics |
| `storage/codec` | WAL payload compression/encoding wrappers such as snappy | Records and indexes |
| `records` | In-memory id to document storage engines | JSON, WAL, HTTP |
| `index` | Index implementations and query execution primitives | HTTP and collection registry |
| `jsonx` or `document` | JSON scan, merge patch, field extraction and typed conversion | Store and records implementations |

Recommended package moves and renames:

| Current | Recommended | Reason |
| --- | --- | --- |
| `collection/stores` | `storage/wal` plus `storage/codec` | Current stores are WAL append/replay implementations and wrappers, not generic document stores. |
| `StoreDisk` | `wal.DiskLog` | It is an append-only WAL, not a general store. |
| `StoreJson` | `wal.JSONLog` or move to test/experimental | It is an alternate WAL encoding. |
| `StoreSnappy` | `codec.SnappyLog` or `wal.Snappy` | It decorates WAL payloads with compression. |
| `StoreAsync` | `wal.AsyncLog` | Async behavior is a durability policy wrapper. |
| `StoreFlusher` | `wal.PeriodicFlusher` | It is a flush policy wrapper. |
| `StoreCrazy` | `internal/experimental` | The name is not product-grade. |
| `collection/index.go` | `index` package | Indexes are large enough to be their own subsystem. |
| `simdscan`, `fson`, `stonejson` | `document/jsonscan` and `internal/experimental` | Keep one supported scanner; keep experiments out of product packages. |
| `Record` | `DocumentSlot` or `RecordSlot` | It stores active/tombstone state plus bytes. |
| `Parsed any` | remove or replace with explicit cache type | The field is unused/unclear and invites arbitrary shared mutable state. |

The desired dependency direction is one-way: HTTP depends on service, service depends on database/collection interfaces, collection depends on records/index/WAL interfaces, and low-level packages depend on nothing product-specific.

### P0: Use Dependency Injection Broadly

The code already has useful interfaces (`stores.Store`, `records.Records[T]`, `collection.Index`), but construction still happens through switches and direct constructors. Replace that with explicit dependency injection and registries. This keeps the fast code fast while making collection-level choices predictable.

Recommended construction model:

```go
type AppDeps struct {
    Clock       Clock
    Logger      Logger
    Metrics     Metrics
    UUID        IDGenerator
    Collections CollectionFactory
    HTTP        HTTPConfig
}

type CollectionFactory interface {
    Open(ctx context.Context, spec CollectionSpec) (*collection.Collection, error)
}

type CollectionDeps struct {
    Log      wal.Log
    Records  records.Engine[collection.DocumentSlot]
    Indexes   index.Registry
    JSON      document.Scanner
    Patcher   document.Patcher
    Defaults  document.DefaultApplier
    Clock     Clock
    UUID      IDGenerator
    Metrics   Metrics
    Logger    Logger
}
```

Recommended registries:

```go
type WALFactory func(WALSpec) (wal.Log, error)
type RecordsFactory func(RecordsSpec) (records.Engine[collection.DocumentSlot], error)
type IndexFactory func(IndexSpec) (index.Index, error)
type CodecFactory func(CodecSpec, wal.Log) (wal.Log, error)
```

Recommended benefits:

| Current issue | DI improvement |
| --- | --- |
| `OpenCollectionCustom` has string switches for raw store, wrapper and records. | Use registered factories with validation and documented names. |
| `Recover` resets records to `RecordsUltra`, ignoring the configured records implementation. | Collection keeps a `RecordsFactory` or records spec and rebuilds the selected engine on recovery. |
| `bootstrap` constructs database, service and API directly. | `bootstrap` builds an `App` from injected dependencies and can be tested without opening sockets. |
| Defaults call `time.Now` and `FastUUID` directly. | Inject `Clock` and `IDGenerator` for deterministic tests and custom ID policies. |
| API handlers return raw errors. | Inject a service interface and central error mapper. |
| Index creation decodes options inside API and collection. | Decode `IndexSpec` once, then use an `IndexRegistry`. |

The first refactor should not introduce a large framework. Use plain constructors and small interfaces. Keep the dependency graph explicit and assembled in one place.

### P0: Make Collection Configuration First-Class

Collection-level configuration should be persisted and recovered as part of the collection catalog, not only passed to `OpenCollectionCustom`. A developer should be able to create different collections optimized for different workloads.

Recommended collection spec:

```json
{
  "name": "events",
  "defaults": {
    "id": "uuid()",
    "created_at": "unixnano()"
  },
  "storage": {
    "wal": "disk",
    "sync": "async",
    "flush_interval": "10s",
    "codec": "snappy"
  },
  "records": {
    "engine": "ultra"
  },
  "indexes": [
    {
      "name": "pk",
      "type": "pk",
      "paths": [["id"]]
    },
    {
      "name": "by_age",
      "type": "btree",
      "fields": ["age"],
      "sparse": true
    }
  ],
  "consistency": {
    "index_updates": "sync"
  }
}
```

Recommended supported knobs:

| Area | Options | Notes |
| --- | --- | --- |
| WAL backend | `disk`, `json`, `memory` | `disk` should be production default; `json` can remain debug-oriented. |
| WAL codec | `none`, `snappy` | Apply at collection level so hot collections can avoid compression. |
| WAL write policy | `direct`, `async`, `periodic_flush` | Make durability implications explicit. |
| Records engine | `ultra`, `hyper`, `turbo`, `fast`, `correct` | Mark one as production default; mark others experimental until documented. |
| Index update policy | `sync`, `async_non_unique` | Avoid hidden eventual consistency. |
| JSON scanner | `simdscan`, `jsonparser` | Prefer `simdscan` for top-level fields; allow fallback for path-heavy workloads if needed. |
| Patch engine | `merge_patch`, `json_patch` | Start with RFC 7396 merge patch; optionally add RFC 6902 later. |

The collection spec should be written to a catalog file or a catalog WAL before opening the collection WAL. Recovery must use the persisted spec, not process defaults. Changing a collection spec should be an explicit operation with validation, because changing records engine or WAL codec has migration implications.

### P0: Rewrite PATCH Around Raw JSON Operations

`Collection.Patch` is correct enough for simple merge patch behavior, and now has an initial compiled-patch/no-op fast path. The remaining inefficient path still parses the full document with `fastjson`, recursively converts patch values from `interface{}` to `fastjson.Value`, marshals values to compare equality and rewrites the whole payload. It also updates indexes and records before WAL append with incomplete rollback.

Recommended PATCH semantics:

| Question | Contract |
| --- | --- |
| Patch format | RFC 7396 JSON Merge Patch for `PATCH /documents/{id}` and bulk patch. |
| Null value | Deletes an object member. |
| Non-object patch | Replaces the whole document. |
| Arrays | Replaced as whole values, not merged item by item. |
| Response | Return updated documents by default for compatibility; allow `return=none` and `return=count`. |
| Index consistency | Same as collection index policy, explicit in collection config. |

Recommended implementation:

| Step | Improvement |
| --- | --- |
| Parse patch once | API should read the request body once and compile it into a `PatchPlan`. Bulk patch should not decode the patch for every document. |
| Keep patch as raw JSON | Avoid `map[string]interface{}` and `interface{}` recursion. Store patch fields as raw byte slices with types from `simdscan.ScanObject`. |
| Fast no-op detection | For each touched top-level field, compare raw canonical or byte-equivalent values before rewriting. |
| Shallow fast path | For patches touching only top-level fields, rewrite the object by splicing existing JSON ranges rather than building a full object tree. |
| Deep fallback | For nested object merge, use a dedicated merge-patch engine that works on raw spans and only parses changed subtrees. |
| Single write pipeline | Compute `oldData`, `newData` and index deltas first; then persist WAL; then publish record and index changes or use a documented optimistic sequence with full rollback. |
| Index-aware delta | Reindex only indexes whose fields may have changed. If patch touches `name`, do not remove/add unrelated `age` indexes. |
| Memory reuse | Use pooled buffers for output construction. |

Recommended internal API:

```go
type PatchPlan struct {
    Kind    PatchKind
    Fields  []PatchField
    Raw     []byte
}

type Patcher interface {
    Compile(raw []byte) (*PatchPlan, error)
    Apply(dst []byte, original []byte, plan *PatchPlan) (updated []byte, changed bool, err error)
    TouchedPaths(plan *PatchPlan) []document.Path
}
```

Recommended collection write pipeline:

```go
func (c *Collection) Patch(ctx context.Context, id int64, plan *PatchPlan, opts WriteOptions) (*WriteResult, error) {
    old := c.records.Get(id)
    updated, changed, err := c.patcher.Apply(c.buffers.Get(), old.Data, plan)
    if err != nil || !changed {
        return result, err
    }

    delta, err := c.indexes.PrepareUpdate(id, old.Data, updated, plan.TouchedPaths())
    if err != nil {
        return nil, err
    }

    if err := c.log.Append(wal.OpUpdate, id, updated, opts.Wait); err != nil {
        return nil, err
    }

    c.records.Set(id, DocumentSlot{Data: updated, Active: true})
    c.indexes.Commit(delta)
    return result, nil
}
```

The key change is that PATCH becomes a compiled operation plus a transactional write path, not ad-hoc JSON tree mutation inside `Collection`.

### P0: Define an Intuitive and Predictable Developer API

The API should make object identity, durability, response shape and index usage predictable. Avoid hidden create-on-insert behavior unless it is explicitly documented as a convenience mode.

Recommended resource model:

| Operation | Endpoint |
| --- | --- |
| Create collection | `PUT /v1/collections/{collection}` |
| Get collection | `GET /v1/collections/{collection}` |
| List collections | `GET /v1/collections` |
| Drop collection | `DELETE /v1/collections/{collection}` |
| Insert documents | `POST /v1/collections/{collection}/documents` |
| Get document | `GET /v1/collections/{collection}/documents/{id}` |
| Replace document | `PUT /v1/collections/{collection}/documents/{id}` |
| Patch document | `PATCH /v1/collections/{collection}/documents/{id}` |
| Delete document | `DELETE /v1/collections/{collection}/documents/{id}` |
| Query documents | `POST /v1/collections/{collection}/query` |
| Bulk patch | `POST /v1/collections/{collection}/documents:patch` |
| Bulk delete | `POST /v1/collections/{collection}/documents:delete` |
| Create index | `PUT /v1/collections/{collection}/indexes/{index}` |
| Get/list/drop index | `GET`/`DELETE /v1/collections/{collection}/indexes/{index}` |

Recommended compatibility strategy: keep current `:insert`, `:find`, `:patch`, `:remove` endpoints through v1 if needed, but document the new resource-oriented endpoints as the stable API. If breaking changes are acceptable, make the resource API v1 and remove the RPC-style endpoints before release.

Recommended query parameters:

| Parameter | Values | Meaning |
| --- | --- | --- |
| `wait` | `true`, `false` | Whether the write waits for the configured durable boundary. |
| `return` | `document`, `none`, `count`, `ids` | Controls response cost and streaming behavior. |
| `consistency` | `default`, `indexed`, `strict` | Whether reads wait for index catch-up where applicable. |
| `limit` | integer | Maximum returned documents. |
| `offset` or `cursor` | integer/string | Pagination. Prefer cursor for stable indexed scans. |

Recommended request examples:

```http
PUT /v1/collections/events
Content-Type: application/json

{
  "defaults": {"id": "uuid()"},
  "storage": {"wal": "disk", "codec": "snappy", "sync": "async"},
  "records": {"engine": "ultra"},
  "consistency": {"index_updates": "sync"}
}
```

```http
POST /v1/collections/events/documents?wait=false&return=document
Content-Type: application/x-ndjson

{"type":"click","user":"u1"}
{"type":"view","user":"u2"}
```

```http
PATCH /v1/collections/events/documents/123?wait=true&return=document
Content-Type: application/merge-patch+json

{"status":"done","temporary_field":null}
```

```http
POST /v1/collections/events/query
Content-Type: application/json

{
  "index": "by_created_at",
  "range": {"gte": [1700000000000], "lt": [1800000000000]},
  "filter": {"type": "click"},
  "limit": 1000
}
```

Recommended response and error shape:

```json
{
  "ok": false,
  "error": {
    "code": "index_conflict",
    "message": "index 'pk' already contains value '123'",
    "details": {"collection": "events", "index": "pk"}
  },
  "request_id": "req_..."
}
```

For streaming endpoints, use NDJSON documents for success and terminate with a non-2xx status only before the first byte is written. If errors can happen mid-stream, emit an explicit NDJSON error frame and document that clients must handle it.

### P1: Refactor Plan

Recommended sequence that minimizes risk:

| Phase | Work | Validation |
| --- | --- | --- |
| 1 | Introduce `CollectionSpec`, factories and registries while keeping existing constructors as adapters. | `make test`, current `cmd/bench` unchanged. |
| 2 | Persist collection specs and use them on recovery/open. | Create collections with different records/WAL codecs, restart, verify same engines are used. |
| 3 | Move WAL wrappers to `storage/wal` and keep type aliases for compatibility during migration. | Store benchmarks and recovery tests. |
| 4 | Extract index package and index registry. | Index unit tests and e2e insert/find/remove. |
| 5 | Extract document scanner/patcher package. | JSON scanner benchmarks and patch correctness tests. |
| 6 | Replace PATCH implementation with compiled raw merge patch. | Patch benchmark allocation budget and e2e patch throughput. |
| 7 | Add resource-oriented HTTP API while keeping old endpoints temporarily. | Acceptance tests for both APIs. |
| 8 | Remove experimental product paths or move them under `internal/experimental`. | Public docs only mention supported engines. |

## Open Source Readiness

Before public release, add:

| Item | Reason |
| --- | --- |
| `CONTRIBUTING.md` | Explain development flow, tests and coding style. |
| `SECURITY.md` | Explain vulnerability reporting. |
| `CODE_OF_CONDUCT.md` | Standard community expectation. |
| Issue templates | Better bug reports and performance reports. |
| Architecture diagrams | Faster onboarding for contributors. |
| Benchmark reproducibility guide | Prevent misleading performance claims. |

## Suggested Roadmap

### Phase 1: Hardening Before Selling

| Priority | Work |
| --- | --- |
| P0 | Define durability, consistency and API contracts. |
| P0 | Add authentication and read/write/admin authorization. |
| P0 | Add DB collection map locking and stop exposing raw maps. |
| P0 | Fix write rollback paths around WAL/index/memory updates. |
| P0 | Introduce `CollectionSpec` and factory-based dependency injection for WAL, codecs, records, indexes and JSON helpers. |
| P0 | Persist collection-level storage and records choices so recovery uses the same engines. |
| P0 | Replace current PATCH with compiled raw JSON merge patch and a transaction-like write pipeline. |
| P0 | Add health/readiness endpoints and structured logging. |
| P0 | Align Go versions across `go.mod`, CI and Dockerfile. |

### Phase 2: Performance Productization

| Priority | Work |
| --- | --- |
| P1 | Standardize `cmd/bench` for HTTP, WAL, indexes and recovery with fixed datasets and variance reporting. |
| P1 | Track p50/p95/p99 latencies and benchmark variance in CI/nightly. |
| P1 | Add metrics for WAL, requests, memory, index lag and recovery. |
| P1 | Reduce patch allocations and validate index update costs. |
| P1 | Implement WAL snapshots or compaction. |
| P1 | Add the resource-oriented HTTP API and keep or deprecate RPC-style endpoints deliberately. |

### Phase 3: Open Source Launch

| Priority | Work |
| --- | --- |
| P1 | Rewrite README as a product landing page. |
| P1 | Add operations, API, performance and architecture docs. |
| P1 | Add contribution and security docs. |
| P2 | Split experimental internals from stable product packages. |
| P2 | Improve UI for index management and operational visibility. |

## Bottom Line

InceptionDB already has enough raw in-memory performance to be credible. The strongest immediate improvements are not more micro-optimizations; they are product guarantees, operational safety, consistency around failure cases, and benchmark coverage that proves real user-facing behavior.

The highest-risk items to fix first are authentication, write rollback correctness, thread-safe collection registry access, explicit durability semantics, and end-to-end performance benchmarks.
