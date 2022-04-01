PROJECT = github.com/fulldump/biff

GOCMD=go

.PHONY: all setup test coverage example

all: test

setup:
	mkdir -p src/$(PROJECT)
	rmdir src/$(PROJECT)
	ln -s ../../.. src/$(PROJECT)

test:
	$(GOCMD) test $(PROJECT)/... -cover

example:
	$(GOCMD) test $(PROJECT)/example -cover

coverage:
	$(GOCMD) test ./src/github.com/fulldump/goconfig -cover -covermode=count -coverprofile=coverage.out; \
	$(GOCMD) tool cover -html=coverage.out
