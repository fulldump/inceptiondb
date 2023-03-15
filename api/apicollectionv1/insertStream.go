package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

// how to try with curl:
// start with tls: HTTPSENABLED=TRUE HTTPSSELFSIGNED=TRUE make run
// curl -v -X POST -T. -k https://localhost:8080/v1/collections/prueba:insert
// type one document and press enter
func insertStream(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		collection, err = s.CreateCollection(collectionName)
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	FullDuplex(w, func(w io.Writer) {

		jsonWriter := json.NewEncoder(w)
		jsonReader := json.NewDecoder(r.Body)

		// w.WriteHeader(http.StatusCreated)

		for {
			item := map[string]interface{}{}
			err := jsonReader.Decode(&item)
			if err == io.EOF {
				// w.WriteHeader(http.StatusCreated)
				return
			}
			if err != nil {
				// TODO: handle error properly
				fmt.Println("ERROR:", err.Error())
				// w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, err = collection.Insert(item)
			if err == nil {
				jsonWriter.Encode(item)
			} else {
				// TODO: handle error properly
				// w.WriteHeader(http.StatusConflict)
				jsonWriter.Encode(err.Error())
			}

		}

	})

	return nil
}

func FullDuplex(w http.ResponseWriter, f func(w io.Writer)) {

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", 500)
		return
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer conn.Close()

	_, err = bufrw.WriteString("HTTP/1.1 202 " + http.StatusText(http.StatusAccepted) + "\r\n")
	w.Header().Write(bufrw)
	_, err = bufrw.WriteString("Transfer-Encoding: chunked\r\n")
	_, err = bufrw.WriteString("\r\n")

	chunkedw := httputil.NewChunkedWriter(bufrw)

	f(chunkedw)

	chunkedw.Close()
	_, err = bufrw.WriteString("\r\n")

	bufrw.Flush()
}
