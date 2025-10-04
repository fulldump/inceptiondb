package apitest

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type Request struct {
	http.Request
	apitest *Apitest
	client  *http.Client
}

func NewRequest(method, urlStr string, a *Apitest) *Request {

	http_request, err := http.NewRequest(method, urlStr, strings.NewReader(""))
	if nil != err {
		panic(err)
	}

	return &Request{*http_request, a, a.client}

}

func (r *Request) WithCredentials(api_key, api_secret string) *Request {

	r.Header.Set("Api-Key", api_key)
	r.Header.Set("Api-Secret", api_secret)

	return r
}

func (r *Request) WithCookie(key, value string) *Request {

	c := &http.Cookie{
		Name:  key,
		Value: value,
	}

	r.AddCookie(c)
	return r
}

func (r *Request) WithHost(host string) *Request {

	r.Host = host

	return r
}

func (r *Request) set_body(body io.Reader) {

	rc, ok := body.(io.ReadCloser)
	if !ok && body != nil {
		rc = ioutil.NopCloser(body)
	}
	r.Body = rc

	if body != nil {
		switch v := body.(type) {
		case *bytes.Buffer:
			r.ContentLength = int64(v.Len())
		case *bytes.Reader:
			r.ContentLength = int64(v.Len())
		case *strings.Reader:
			r.ContentLength = int64(v.Len())
		}
	}
}

func (r *Request) WithHeader(key, value string) *Request {

	r.Header.Set(key, value)

	return r
}

func (r *Request) WithQuery(key, value string) *Request {

	values := r.URL.Query()
	values.Add(key, value)
	r.URL.RawQuery = values.Encode()

	return r
}

func (r *Request) WithBodyString(body string) *Request {
	b := strings.NewReader(body)
	r.set_body(b)

	return r
}

func (r *Request) WithBodyJson(o interface{}) *Request {
	bytes, err := json.Marshal(o)
	if nil != err {
		panic(err)
	}

	r.WithBodyString(string(bytes))

	return r
}

func (r *Request) WithHttpClient(client *http.Client) *Request {

	r.client = client

	return r
}

func (r *Request) Do() *Response {

	tee := &tee{Buffer: r.Request.Body}
	r.Request.Body = tee

	res, err := r.client.Do(&r.Request)
	if err != nil {
		panic(err)
	}

	return &Response{Response: *res, response_body_bytes: nil, request_body_bytes: tee.Bytes}
}

func (r *Request) DoAsync(f func(*Response)) {
	c := <-r.apitest.clients

	response, err := c.Do(&r.Request)
	if err != nil {
		panic(err)
	}
	wresponse := &Response{*response, r.apitest, c, nil, nil}
	f(wresponse)

	wresponse.BodyClose()

	r.apitest.clients <- c
}
