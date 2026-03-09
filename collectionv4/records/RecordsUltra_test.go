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
