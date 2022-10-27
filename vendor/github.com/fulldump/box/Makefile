PROJECT = github.com/fulldump/box

GOCMD=go

.PHONY: all setup test coverage example

all: info test

setup:
	mkdir -p src/$(PROJECT)
	rmdir src/$(PROJECT)
	ln -s ../../.. src/$(PROJECT)

info:
	$(GOCMD) version
	$(GOCMD) env

test:
	$(GOCMD) test -cover $(PROJECT)/...

example:
	$(GOCMD) install $(PROJECT)/example

coverage:
	$(GOCMD) test $(PROJECT) -cover -covermode=count -coverprofile=coverage.out; \
	$(GOCMD) tool cover -html=coverage.out
