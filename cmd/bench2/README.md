# InceptionDB Bench Tool

## How to use

Compile and run the command.

## Test inserts

```sh
go run . --test insert --n 2_000_000 --workers 16
```

## Test patch

```sh
go run . --test patch --n 100_000 --workers 16 
```
