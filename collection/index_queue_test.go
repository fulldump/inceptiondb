package collection

import (
	"sync"
	"testing"
)

func TestIndexQueue_OrderAndDrainOnClose(t *testing.T) {
	q := newIndexQueue()

	var got []int64
	block := make(chan struct{})
	go q.run(func(batch []asyncIndexReq) {
		<-block // let requests pile up so close finds them pending
		for _, r := range batch {
			got = append(got, r.id)
		}
	})

	const producers, perProducer = 8, 1000
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(p int) {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				q.push(asyncIndexReq{id: int64(p*perProducer + i)})
			}
		}(p)
	}
	wg.Wait()

	close(block)
	q.close() // must process everything pushed before returning

	if len(got) != producers*perProducer {
		t.Fatalf("processed %d requests, want %d", len(got), producers*perProducer)
	}
	last := map[int64]int64{}
	for _, id := range got {
		p := id / perProducer
		if prev, ok := last[p]; ok && id <= prev {
			t.Fatalf("producer %d: request %d processed after %d", p, id, prev)
		}
		last[p] = id
	}

	q.push(asyncIndexReq{id: -1}) // after close: dropped, must not block or panic
}
