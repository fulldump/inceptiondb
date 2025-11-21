package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/fulldump/goconfig"
)

type Config struct {
	Test    string `usage:"name of the test: ALL | INSERT | PATCH"`
	Base    string `usage:"base URL"`
	N       int64  `usage:"number of documents"`
	Workers int    `usage:"number of workers"`
}

var cleanups []func()

func main() {

	defer func() {
		fmt.Println("Cleaning up...")
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	c := Config{
		Test:    "remove",
		Base:    "",
		N:       1_000_000,
		Workers: 16,
	}
	goconfig.Read(&c)

	switch strings.ToUpper(c.Test) {
	case "ALL":
	case "INSERT":
		TestInsert(c)
	case "PATCH":
		TestPatch(c)
	case "REMOVE":
		TestRemove(c)
	default:
		log.Fatalf("Unknown test %s", c.Test)
	}

}
