PROJECT=github.com/fulldump/apitest

GO = GOFLAGS=-mod=vendor go

.PHONY: test
test:
	$(GO) test $(PROJECT)/... --cover
