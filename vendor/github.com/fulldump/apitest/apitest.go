package apitest

import (
	"crypto/tls"
	"net/http/httptest"

	"net/http"
)

type Apitest struct {
	Handler http.Handler      // handler to test
	Server  *httptest.Server  // testing server
	Base    string            // Base uri to make requests
	client  *http.Client      // Default Http Client to use in requests
	clients chan *http.Client // http clients
}

// Deprecated: Please, use NewWithHandler instead
func New(h http.Handler) *Apitest {

	return NewWithPool(h, 2)
}

func NewWithBase(base string) *Apitest {
	return &Apitest{
		client: http.DefaultClient,
		Base:   base,
	}
}

func NewWithHandler(h http.Handler) *Apitest {
	return NewWithPool(h, 2)
}

func NewWithPool(h http.Handler, n int) *Apitest {

	s := httptest.NewServer(h)

	a := &Apitest{
		Base:    s.URL,
		Handler: h,
		Server:  s,
		clients: make(chan *http.Client, n),
		client:  http.DefaultClient,
	}

	for i := 0; i < cap(a.clients); i++ {
		a.clients <- &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				DisableKeepAlives: false,
			},
		}
	}

	return a
}

func (a *Apitest) WithHttpClient(client *http.Client) *Apitest {
	a.client = client
	a.clients = make(chan *http.Client, cap(a.clients))
	if a.clients != nil {
		for i := 0; i < cap(a.clients); i++ {
			a.clients <- client
		}
	}
	return a
}

func (a *Apitest) Destroy() {
	if nil != a.Server {
		a.Server.Close()
		a.Server = nil
	}
}

func (a *Apitest) Request(method, path string) *Request {

	return NewRequest(
		method,
		a.Base+path,
		a,
	)
}
