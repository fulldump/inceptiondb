package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// durability is set on every collection created by the benchmark.
var durability string

// tempBase is where TempDir creates data directories. Note that /tmp is often
// a tmpfs: use a directory on a real disk to measure durability.
var tempBase string

// httpAddr overrides the address of the embedded server.
var httpAddr string

func TempDir() (string, func()) {
	dir, err := os.MkdirTemp(tempBase, "inceptiondb_bench_*")
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

	payload, _ := json.Marshal(JSON{"name": name, "durability": durability})

	req, _ := http.NewRequest("POST", base+"/v1/collections", bytes.NewReader(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	io.Copy(os.Stdout, resp.Body)

	return name
}

func CreatePKIndex(base, collectionName string) {
	payload, _ := json.Marshal(JSON{
		"name":  "pk",
		"type":  "pk",
		"paths": [][]string{{"id"}},
	})

	req, _ := http.NewRequest("POST", base+"/v1/collections/"+collectionName+":createIndex", bytes.NewReader(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		panic(fmt.Sprintf("create pk index failed: status=%s body=%s", resp.Status, string(body)))
	}
}

func CreateBtreeIndex(base, collectionName string) {
	payload, _ := json.Marshal(JSON{
		"name":   "btree_age",
		"type":   "btree",
		"fields": []string{"age"},
	})

	req, _ := http.NewRequest("POST", base+"/v1/collections/"+collectionName+":createIndex", bytes.NewReader(payload))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		panic(fmt.Sprintf("create btree index failed: status=%s body=%s", resp.Status, string(body)))
	}
}

func CreateServer(c *Config) (start, stop func()) {
	dir, cleanup := TempDir()
	cleanups = append(cleanups, cleanup)

	conf := configuration.Default()
	conf.Dir = dir
	if httpAddr != "" {
		conf.HttpAddr = httpAddr
	}
	c.Base = "http://" + conf.HttpAddr

	return bootstrap.Bootstrap(conf)
}
