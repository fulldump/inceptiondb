
VERSION = $(shell git describe --tags --always)
FLAGS = -ldflags "\
  -X main.VERSION=$(VERSION) \
"

test:
	go test -cover ./...

run:
	STATICS=statics/www/ go run $(FLAGS) ./cmd/inceptiondb/...

build:
	go build $(FLAGS) -o bin/ ./cmd/inceptiondb/...

.PHONY: release
release: clean
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(FLAGS) -o bin/inceptiondb.linux.arm64 ./cmd/...
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(FLAGS) -o bin/inceptiondb.linux.amd64 ./cmd/...
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build $(FLAGS) -o bin/inceptiondb.win.arm64.exe ./cmd/...
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(FLAGS) -o bin/inceptiondb.win.amd64.exe ./cmd/...
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(FLAGS) -o bin/inceptiondb.mac.arm64 ./cmd/...
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(FLAGS) -o bin/inceptiondb.mac.amd64 ./cmd/...
	md5sum bin/inceptiondb.* > bin/checksum-md5
	sha256sum bin/inceptiondb.* > bin/checksum-sha256

.PHONY: clean
clean:
	rm -f bin/*

.PHONY: deps
deps:
	go mod tidy -v;
	go mod download;
	go mod vendor;

.PHONY: doc
doc:
	go clean -testcache
	API_EXAMPLES_PATH="../doc/examples" go test ./api/...

.PHONY: book
book:
	mdbook build -d ../../statics/www/book/ ./doc/book/

.PHONY: version
version:
	@echo $(VERSION)
