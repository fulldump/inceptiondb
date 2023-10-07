package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func main() {

	r, w := io.Pipe()

	// d := json.NewDecoder(os.Stdin)
	e := json.NewEncoder(w)

	go func() {
		for i := 0; i < 1000; i++ {

			e.Encode(map[string]any{
				"id":      i,
				"payload": strings.Repeat("fake ", 1000),
			})

			fmt.Println("sent", i)

			// time.Sleep(10 * time.Millisecond)
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

		req, err := http.NewRequest("POST", "http://localhost:8080/v1/collections/streammm:insertFullduplex", r)
		if err != nil {
			fmt.Println("ERROR: new request:", err.Error())
			os.Exit(3)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Println("ERROR: do request:", err.Error())
			os.Exit(4)
		}

		d := json.NewDecoder(resp.Body)

		var o json.RawMessage

		for {
			err := d.Decode(&o)
			if err != nil {
				fmt.Println("ERROR: response body:", err.Error())
				os.Exit(5)
			}

			fmt.Println("RECEIVED:", string(o))
		}

	}
}
