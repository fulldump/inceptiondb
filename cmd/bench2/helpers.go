package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/configuration"
)

type JSON = map[string]any

func Parallel(workers int, f func()) {
	wg := &sync.WaitGroup{}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f()
		}()
	}
	wg.Wait()
}

func TempDir() (string, func()) {
	dir, err := os.MkdirTemp("", "inceptiondb_bench_*")
	if err != nil {
		panic("Could not create temp directory: " + err.Error())
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

func CreateCollection(base string) string {

	name := "col-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	payload, _ := json.Marshal(JSON{"name": name})

	req, _ := http.NewRequest("POST", base+"/v1/collections", bytes.NewReader(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	io.Copy(os.Stdout, resp.Body)

	return name
}

func CreateServer(c *Config) (start, stop func()) {
	dir, cleanup := TempDir()
	cleanups = append(cleanups, cleanup)

	conf := configuration.Default()
	conf.Dir = dir
	c.Base = "http://" + conf.HttpAddr

	return bootstrap.Bootstrap(conf)
}
