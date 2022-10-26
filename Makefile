
test:
	go test ./...

run:
	go run .

build:
	go build -o bin/ .

.PHONY: deps
deps:
	go mod tidy -v;
	go mod download;
	go mod vendor;

.PHONY: doc
doc:
	go clean -testcache
	API_EXAMPLES_PATH="./examples" go test ./api/...
