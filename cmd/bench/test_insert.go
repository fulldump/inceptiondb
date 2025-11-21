package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"sync/atomic"
	"time"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/collectionv2"
	"github.com/fulldump/inceptiondb/configuration"
)

func TestInsert(c Config) {

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
	// if c.Base == "" {
	// 		start, stop = CreateServer(&c)
	// 		defer stop()
	// 		go start()
	// 	}

	collection := CreateCollection(c.Base)

	payload := strings.Repeat("fake ", 0)
	_ = payload

	client := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     1024,
			MaxIdleConnsPerHost: 1024,
			MaxIdleConns:        1024,
		},
	}

	items := c.N

	go func() {
		for {
			fmt.Println("items:", items)
			time.Sleep(1 * time.Second)
		}
	}()

	t0 := time.Now()
	Parallel(c.Workers, func() {

		r, w := io.Pipe()

		wb := bufio.NewWriterSize(w, 1*1024*1024)

		go func() {
			for {
				n := atomic.AddInt64(&items, -1)
				if n < 0 {
					break
				}
				fmt.Fprintf(wb, "{\"id\":%d,\"n\":\"%d\"}\n", n, n)
			}
			wb.Flush()
			w.Close()
		}()

		req, err := http.NewRequest("POST", c.Base+"/v1/collections/"+collection+":insert", r)
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

	took := time.Since(t0)
	fmt.Println("sent:", c.N)
	fmt.Println("took:", took)
	fmt.Printf("Throughput: %.2f rows/sec\n", float64(c.N)/took.Seconds())

	if createServer {
		stop() // Stop the server

		t1 := time.Now()
		collectionv2.OpenCollection(path.Join(dataDir, collection))
		tookOpen := time.Since(t1)
		fmt.Println("open took:", tookOpen)
		fmt.Printf("Throughput Open: %.2f rows/sec\n", float64(c.N)/tookOpen.Seconds())
	}

}
