package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

type Response struct {
	http.Response
	apitest             *Apitest
	client              *http.Client
	response_body_bytes []byte
	request_body_bytes  []byte
}

func (r *Response) BodyClose() {
	io.Copy(ioutil.Discard, r.Body)
	r.Body.Close()
}

func (r *Response) BodyRequestBytes() []byte {
	return r.request_body_bytes
}

func (r *Response) BodyRequestString() string {
	return string(r.BodyRequestBytes())
}

func (r *Response) BodyBytes() []byte {

	if nil == r.response_body_bytes {
		body, err := ioutil.ReadAll(r.Body)

		r.response_body_bytes = body

		if err != nil {
			panic(err)
		}
		r.BodyClose()

	}

	return r.response_body_bytes
}

func (r *Response) BodyString() string {
	return string(r.BodyBytes())
}

func (r *Response) BodyJson() interface{} {

	b := bytes.NewBuffer(r.BodyBytes())

	d := json.NewDecoder(b)
	d.UseNumber()
	var body interface{}
	if err := d.Decode(&body); err != nil {
		panic(err)
	}
	r.BodyClose()
	return body
}

func (r *Response) BodyJsonMap() map[string]interface{} {

	b := bytes.NewBuffer(r.BodyBytes())

	d := json.NewDecoder(b)
	d.UseNumber()
	body := map[string]interface{}{}
	if err := d.Decode(&body); err != nil {
		panic(err)
	}
	r.BodyClose()
	return body
}
