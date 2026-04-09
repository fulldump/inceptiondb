package collectionv2

import (
	"os"
	"sync"
	"testing"
	"time"
)

func TestRaceInsertTraverse(t *testing.T) {
	filename := "/tmp/race_test_collection_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	start := time.Now()
	duration := 2 * time.Second

	// Writer
	go func() {
		defer wg.Done()
		i := 0
		for time.Since(start) < duration {
			_, err := c.Insert(map[string]any{"v": i})
			if err != nil {
				t.Error(err)
				return
			}
			i++
		}
	}()

	// Reader
	go func() {
		defer wg.Done()
		for time.Since(start) < duration {
			c.Traverse(func(data []byte) {
				// just read
			})
		}
	}()

	wg.Wait()
}
