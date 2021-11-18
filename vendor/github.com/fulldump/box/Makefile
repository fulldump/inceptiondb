PROJECT = github.com/fulldump/box

GOCMD=GOPATH=`pwd` go

.PHONY: all setup test coverage example

all: test

setup:
	mkdir -p src/$(PROJECT)
	rmdir src/$(PROJECT)
	ln -s ../../.. src/$(PROJECT)

test:
	$(GOCMD) test $(PROJECT) -cover

example:
	$(GOCMD) test $(PROJECT)/example -cover

coverage:
	$(GOCMD) test ./src/$(PROJECT) -cover -covermode=count -coverprofile=coverage.out; \
	$(GOCMD) tool cover -html=coverage.out
