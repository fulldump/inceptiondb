package streamtest

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/configuration"
)

type Item struct {
	Id      int64  `json:"id"`
	Payload string `json:"payload"`
}

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

func Test_Streamtest(t *testing.T) {

	t.SkipNow()

	if false {
		conf := configuration.Default()
		conf.Dir = t.TempDir()

		start, stop := bootstrap.Bootstrap(conf)
		defer stop()

		go start()
	}

	base := "https://inceptiondb.io"
	base = "http://localhost:8080"

	{
		// Create collection
		payload := strings.NewReader(`{"name": "streammm"}`)
		req, _ := http.NewRequest("POST", base+"/v1/collections", payload)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(body))
	}

	counter := int64(0)
	t0 := time.Now()
	load_per_worker := 100_000

	payload := strings.Repeat("fake ", 0)
	_ = payload

	c := &http.Client{
		Transport: &http.Transport{
			MaxConnsPerHost:     1024,
			MaxIdleConnsPerHost: 1024,
			MaxIdleConns:        1024,
		},
	}

	Parallel(16, func() {

		r, w := io.Pipe()

		wb := bufio.NewWriterSize(w, 1*1024*1024)

		go func() {
			// e := json.NewEncoder(w)
			for i := 0; i < load_per_worker; i++ {
				// e.Encode(Item{
				// 	Id:      atomic.AddInt64(&counter, 1),
				// 	Payload: payload,
				// })
				n := atomic.AddInt64(&counter, 1)
				fmt.Fprintf(wb, "{\"id\":%d,\"n\":\"%d\"}\n", n, n)
			}
			wb.Flush()
			w.Close()
		}()

		{
			req, err := http.NewRequest("POST", base+"/v1/collections/streammm:insert", r)
			if err != nil {
				fmt.Println("ERROR: new request:", err.Error())
				os.Exit(3)
			}

			resp, err := c.Do(req)
			if err != nil {
				fmt.Println("ERROR: do request:", err.Error())
				os.Exit(4)
			}
			io.Copy(io.Discard, resp.Body)
		}
	})

	took := time.Since(t0)
	fmt.Println("received:", counter)
	fmt.Println("took:", took)
	fmt.Printf("Throughput: %.2f rows/sec\n", float64(counter)/took.Seconds())

}
