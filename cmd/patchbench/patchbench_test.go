package patchbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fulldump/inceptiondb/bootstrap"
	"github.com/fulldump/inceptiondb/configuration"
)

func BenchmarkPatch(b *testing.B) {
	b.ReportAllocs()

	conf := configuration.Default()
	conf.Dir = b.TempDir()
	conf.HttpAddr = "127.0.0.1:18080"
	conf.ShowBanner = false

	start, stop := bootstrap.Bootstrap(conf)
	defer stop()

	go start()

	baseURL := "http://" + conf.HttpAddr
	collectionName := "patch-benchmark"

	transport := &http.Transport{
		MaxConnsPerHost:     1024,
		MaxIdleConns:        1024,
		MaxIdleConnsPerHost: 1024,
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	ensureCollection(b, client, baseURL, collectionName)

	const datasetSize = 1024
	preloadDocuments(b, client, baseURL, collectionName, datasetSize)

	patchURL := fmt.Sprintf("%s/v1/collections/%s:patch", baseURL, collectionName)

	var opCounter int64

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			op := atomic.AddInt64(&opCounter, 1)
			targetID := int(op % datasetSize)
			patchValue := op

			body := fmt.Sprintf(`{"filter":{"id":"%s"},"patch":{"value":%d}}`, strconv.Itoa(targetID), patchValue)
			req, err := http.NewRequest(http.MethodPost, patchURL, strings.NewReader(body))
			if err != nil {
				b.Fatalf("new request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				b.Fatalf("do request: %v", err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				b.Fatalf("unexpected status: %d", resp.StatusCode)
			}
		}
	})
}

func ensureCollection(b *testing.B, client *http.Client, baseURL, name string) {
	b.Helper()

	endpoint := baseURL + "/v1/collections"
	payload := fmt.Sprintf(`{"name":"%s"}`, name)

	var lastErr error
	for i := 0; i < 100; i++ {
		req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(payload))
		if err != nil {
			b.Fatalf("ensure collection request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(100 * time.Millisecond)
			continue
		}

		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
			return
		}

		lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
		time.Sleep(100 * time.Millisecond)
	}

	if lastErr != nil {
		b.Fatalf("ensure collection: %v", lastErr)
	}
	b.Fatalf("ensure collection: timeout waiting for server")
}

func preloadDocuments(b *testing.B, client *http.Client, baseURL, collection string, size int) {
	b.Helper()

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for i := 0; i < size; i++ {
		doc := map[string]interface{}{
			"id":    strconv.Itoa(i),
			"value": 0,
		}
		if err := enc.Encode(doc); err != nil {
			b.Fatalf("marshal doc: %v", err)
		}
	}

	endpoint := fmt.Sprintf("%s/v1/collections/%s:insert", baseURL, collection)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(buf.Bytes()))
	if err != nil {
		b.Fatalf("preload request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		b.Fatalf("preload request: %v", err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b.Fatalf("preload unexpected status: %d", resp.StatusCode)
	}
}
