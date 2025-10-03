package streamtest

import (
	"bytes"
	"testing"

	"github.com/fulldump/biff"
	json2 "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

func Test_Jsonv2(t *testing.T) {
	b := bytes.NewBufferString(`{"hello":"world"}`)
	jsonDecoder := jsontext.NewDecoder(b)
	greeting := struct {
		Hello string `json:"hello"`
	}{
		Hello: "Trololo",
	}
	err := json2.UnmarshalDecode(jsonDecoder, &greeting)

	biff.AssertNil(err)
	biff.AssertEqual(greeting.Hello, "world")
}
