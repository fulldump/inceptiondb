package main

import (
	"fmt"
	"log"
	"os"
	"runtime/pprof"
	"strings"

	"github.com/fulldump/goconfig"
)

type Config struct {
	Test    string `usage:"name of the test: ALL | INSERT | INSERTPK | INSERTBTREE | RETRIEVEBTREE | RETRIEVEFS | PATCH | REMOVE"`
	Base    string `usage:"base URL"`
	N       int64  `usage:"number of documents"`
	Workers int    `usage:"number of workers"`
}

var cleanups []func()

func main() {
	if os.Getenv("PPROF") != "" {
		f, err := os.Create("cpu.prof")
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		cleanups = append(cleanups, pprof.StopCPUProfile)
	}

	defer func() {
		fmt.Println("Cleaning up...")
		for _, cleanup := range cleanups {
			cleanup()
		}
	}()

	c := Config{
		Test:    "insert",
		Base:    "",
		N:       1_000_000,
		Workers: 16,
	}
	goconfig.Read(&c)

	switch strings.ToUpper(c.Test) {
	case "ALL":
	case "INSERT":
		TestInsert(c)
	case "INSERTPK":
		TestInsertPK(c)
	case "INSERTBTREE":
		TestInsertBtree(c)
	case "RETRIEVEBTREE":
		TestRetrieveBtree(c)
	case "RETRIEVEFS":
		TestRetrieveFS(c)
	case "PATCH":
		TestPatch(c)
	case "REMOVE":
		TestRemove(c)
	default:
		log.Fatalf("Unknown test %s", c.Test)
	}

}
