# InceptionDB Bench Tool

## How to use

Compile and run the command.

## Test inserts

```sh
go run . --test insert --n 2_000_000 --workers 16
```

## Test inserts with PK index

```sh
go run . --test insertpk --n 2_000_000 --workers 16
```

## Test patch

```sh
go run . --test patch --n 100_000 --workers 16 
```

## Test remove

```sh
go run . --test remove --n 1_000_000 --workers 16 
```

## Run all benchmark scenarios

```sh
go run . --test all --n 1_000_000 --workers 16
```

This runs insert, insert with PK, insert with BTree, BTree retrieval,
full-scan retrieval, patch and remove scenarios. Each scenario starts the
service when no `--base` URL is provided and registers cleanup for temporary
data.
