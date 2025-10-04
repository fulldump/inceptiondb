package streamtest

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

	// t.SkipNow()

	counter := int64(0)
	t0 := time.Now()
	load_per_worker := 100_000

	payload := strings.Repeat("fake ", 0)
	_ = payload

	Parallel(16, func() {

		r, w := io.Pipe()

		// wb := bufio.NewWriterSize(w, 1*1024*1024)

		go func() {
			// e := json.NewEncoder(w)
			for i := 0; i < load_per_worker; i++ {
				// e.Encode(Item{
				// 	Id:      atomic.AddInt64(&counter, 1),
				// 	Payload: payload,
				// })
				n := atomic.AddInt64(&counter, 1)
				fmt.Fprintf(w, "{\"id\":%d,\"n\":\"%d\"}\n", n, n)
			}
			// wb.Flush()
			w.Close()
		}()

		{

			u := "http://localhost:8080/v1/collections/streammm:insertFullduplex"
			u = "https://inceptiondb.io/v1/collections/streammm:insert"
			u = "http://localhost:8080/v1/collections/streammm:insert"
			// u = "http://inceptiondb.io:8080/v1/collections/streammm:insertFullduplex"

			req, err := http.NewRequest("POST", u, r)
			if err != nil {
				fmt.Println("ERROR: new request:", err.Error())
				os.Exit(3)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Println("ERROR: do request:", err.Error())
				os.Exit(4)
			}
			io.Copy(io.Discard, resp.Body)
		}
	})

	fmt.Println("received:", counter)
	fmt.Println("took:", time.Since(t0))

}
