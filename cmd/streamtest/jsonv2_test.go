package streamtest

import (
	"bytes"
	"encoding/json/jsontext"
	json2 "encoding/json/v2"
	"testing"

	"github.com/fulldump/biff"
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
