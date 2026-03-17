package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/configuration"
)

func TestRetrieveFS(c Config) {

	createServer := c.Base == ""

	var start, stop func()
	if createServer {
		dir, cleanup := TempDir()
		cleanups = append(cleanups, cleanup)

		conf := configuration.Default()
		conf.Dir = dir
		conf.HttpAddr = "127.0.0.1:8084"
		c.Base = "http://" + conf.HttpAddr

		start, stop = bootstrap.Bootstrap(conf)
		go start()
	}

	collectionName := CreateCollection(c.Base)

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     1024,
			MaxIdleConnsPerHost: 1024,
			MaxIdleConns:        1024,
		},
	}

	// Phase 1: Insert data (no indexes)
	fmt.Println("=== Phase 1: Inserting data (no indexes) ===")
	insertItems := c.N
	t0 := time.Now()

	Parallel(c.Workers, func() {
		r, w := io.Pipe()
		wb := bufio.NewWriterSize(w, 1*1024*1024)

		go func() {
			for {
				n := atomic.AddInt64(&insertItems, -1)
				if n < 0 {
					break
				}
				even := n%2 == 0
				fmt.Fprintf(wb, "{\"id\":%d,\"name\":\"user-%d\",\"age\":%d,\"even\":%t}\n", n, n, n, even)
			}
			wb.Flush()
			w.Close()
		}()

		req, err := http.NewRequest("POST", c.Base+"/v1/collections/"+collectionName+":insert", r)
		if err != nil {
			fmt.Println("ERROR: new request:", err.Error())
			os.Exit(3)
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERROR: do request:", err.Error())
			os.Exit(4)
		}
		io.Copy(io.Discard, resp.Body)
	})

	insertTook := time.Since(t0)
	fmt.Printf("Inserted %d docs in %s (%.2f rows/sec)\n", c.N, insertTook, float64(c.N)/insertTook.Seconds())

	time.Sleep(2 * time.Second)

	// Phase 2: Full scan without filter
	fmt.Println("=== Phase 2: Full scan (no filter) ===")
	{
		payload, _ := json.Marshal(JSON{
			"limit": c.N,
		})
		fmt.Println(string(payload))

		req, err := http.NewRequest("POST", c.Base+"/v1/collections/"+collectionName+":find", strings.NewReader(string(payload)))
		if err != nil {
			fmt.Println("ERROR: new request:", err.Error())
			os.Exit(3)
		}

		t1 := time.Now()

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERROR: do request:", err.Error())
			os.Exit(4)
		}

		docs := int64(0)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)
		for scanner.Scan() {
			docs++
		}
		resp.Body.Close()

		took := time.Since(t1)
		fmt.Println("docs retrieved:", docs)
		fmt.Println("took:", took)
		fmt.Printf("Throughput: %.2f docs/sec\n", float64(docs)/took.Seconds())
	}

	// Phase 3: Full scan with filter (returns ~half of the collection)
	fmt.Println("=== Phase 3: Full scan with filter (even=true, ~50%%) ===")
	{
		payload, _ := json.Marshal(JSON{
			"filter": JSON{"even": true},
			"limit":  c.N,
		})
		fmt.Println(string(payload))

		req, err := http.NewRequest("POST", c.Base+"/v1/collections/"+collectionName+":find", strings.NewReader(string(payload)))
		if err != nil {
			fmt.Println("ERROR: new request:", err.Error())
			os.Exit(3)
		}

		t2 := time.Now()

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERROR: do request:", err.Error())
			os.Exit(4)
		}

		docs := int64(0)
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)
		for scanner.Scan() {
			docs++
		}
		resp.Body.Close()

		took := time.Since(t2)
		fmt.Println("docs retrieved:", docs)
		fmt.Println("took:", took)
		fmt.Printf("Throughput: %.2f docs/sec\n", float64(docs)/took.Seconds())
	}

	if createServer {
		stop()
	}
}
