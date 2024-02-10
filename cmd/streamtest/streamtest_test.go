package streamtest

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func Test_Streamtest(t *testing.T) {

	t.Skip()

	r, w := io.Pipe()

	// d := json.NewDecoder(os.Stdin)
	e := json.NewEncoder(w)

	go func() {

		payload := strings.Repeat("fake ", 10)
		for i := 0; i < 1_000_000; i++ {

			e.Encode(map[string]any{
				"id":      i,
				"payload": payload,
			})

			fmt.Println("sent", i)

			// time.Sleep(1000 * time.Millisecond)
		}
		w.Close()
	}()

	// go func() {
	//
	// 	var o json.RawMessage
	//
	// 	for {
	// 		err := d.Decode(&o)
	// 		if err != nil {
	// 			fmt.Println("ERROR: stdin:", err.Error())
	// 			os.Exit(1)
	// 		}
	//
	// 		fmt.Println("READ:", string(o))
	// 		err = e.Encode(o)
	// 		if err != nil {
	// 			fmt.Println("ERROR: encode:", err.Error())
	// 			os.Exit(2)
	// 		}
	// 	}
	// }()

	{

		u := "http://localhost:8080/v1/collections/streammm:insertFullduplex"
		// u = "https://inceptiondb.io/v1/collections/streammm:insertFullduplex"
		// u = "http://localhost:8080/v1/collections/streammm:insert"
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

		// io.Copy(os.Stdout, resp.Body)
		// fmt.Println("")

		d := json.NewDecoder(resp.Body)
		receivedCounter := 0
		var o json.RawMessage
		for {
			err := d.Decode(&o)
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println("ERROR: response body:", err.Error())
				os.Exit(5)
			}

			fmt.Println("RECEIVED:", string(o))
			receivedCounter++
		}

		fmt.Println("received:", receivedCounter)
	}
}
