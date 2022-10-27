package apitest

import "io"

type tee struct {
	Buffer io.ReadCloser
	Bytes  []byte
}

func (t *tee) Read(p []byte) (n int, err error) {

	n, err = t.Buffer.Read(p)

	t.Bytes = append(t.Bytes, p[:n]...)

	return
}

func (t *tee) Close() (err error) {
	return t.Buffer.Close()

}
