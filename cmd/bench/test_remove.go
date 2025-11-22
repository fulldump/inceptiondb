package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/collectionv2"
	"github.com/fulldump/inceptiondb/configuration"
)

func TestRemove(c Config) {

	createServer := c.Base == ""

	var start, stop func()
	var dataDir string
	if createServer {
		dir, cleanup := TempDir()
		dataDir = dir
		cleanups = append(cleanups, cleanup)

		conf := configuration.Default()
		conf.Dir = dir
		c.Base = "http://" + conf.HttpAddr

		start, stop = bootstrap.Bootstrap(conf)
		go start()
	}

	collectionName := CreateCollection(c.Base)

	transport := &http.Transport{
		MaxConnsPerHost:     1024,
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	{
		fmt.Println("Preload documents...")
		r, w := io.Pipe()

		encoder := json.NewEncoder(w)
		go func() {
			for i := int64(0); i < c.N; i++ {
				encoder.Encode(JSON{
					"id":     strconv.FormatInt(i, 10),
					"value":  0,
					"worker": i % int64(c.Workers),
				})
			}
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
	}

	removeURL := fmt.Sprintf("%s/v1/collections/%s:remove", c.Base, collectionName)

	t0 := time.Now()
	worker := int64(-1)
	Parallel(c.Workers, func() {
		w := atomic.AddInt64(&worker, 1)

		// Remove all documents belonging to this worker
		body := fmt.Sprintf(`{"filter":{"worker":%d},"limit":-1}`, w)
		req, err := http.NewRequest(http.MethodPost, removeURL, strings.NewReader(body))
		if err != nil {
			fmt.Println("ERROR: new request:", err.Error())
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERROR: do request:", err.Error())
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Println("ERROR: bad status:", resp.Status)
		}
	})

	took := time.Since(t0)
	fmt.Println("removed:", c.N)
	fmt.Println("took:", took)
	fmt.Printf("Throughput: %.2f rows/sec\n", float64(c.N)/took.Seconds())

	if !createServer {
		return
	}

	stop() // Stop the server

	t1 := time.Now()
	collectionv2.OpenCollection(path.Join(dataDir, collectionName))
	tookOpen := time.Since(t1)
	fmt.Println("open took:", tookOpen)
	fmt.Printf("Throughput Open: %.2f rows/sec\n", float64(c.N)/tookOpen.Seconds())
}
