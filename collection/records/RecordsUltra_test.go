package records

import (
	"fmt"
	"testing"
)

func Test_RecordsUltra_Suite(t *testing.T) {
	RunRecordsSuite(t, func() Records[int] {
		return NewRecordsUltra[int]()
	})
}

func Benchmark_RecordsUltra_Stress(b *testing.B) {
	for _, workers := range []int{16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			RunConcurrentMixedOperationsBenchmark(b, workers, func() Records[int] {
				return NewRecordsUltra[int]()
			})
		})
	}
}

func Benchmark_RecordsUltra_Insert(b *testing.B) {
	for _, workers := range []int{16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			RunConcurrentInsertBenchmark(b, workers, func() Records[int] {
				return NewRecordsUltra[int]()
			})
		})
	}
}

func Benchmark_RecordsUltra_Set(b *testing.B) {
	for _, workers := range []int{16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			RunConcurrentSetBenchmark(b, workers, func() Records[int] {
				return NewRecordsUltra[int]()
			})
		})
	}
}

func Benchmark_RecordsUltra_Traverse(b *testing.B) {
	RunBenchmarkTraverse(b, func() Records[int] {
		return NewRecordsUltra[int]()
	})
}

// A single insert stream must spread over the shards, otherwise every later
// read and write of the collection contends on one shard lock.
func Test_RecordsUltra_SingleStreamUsesAllShards(t *testing.T) {
	r := NewRecordsUltra[int]()
	counts := map[int64]int{}
	for i := 0; i < recordsUltraNumShards*recordsUltraPickerRun; i++ {
		counts[r.Insert(i)&recordsUltraShardMask]++
	}
	if len(counts) < recordsUltraNumShards/2 {
		t.Fatalf("single stream used %d of %d shards", len(counts), recordsUltraNumShards)
	}
}
