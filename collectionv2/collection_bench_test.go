package collectionv2

import (
"strconv"
"testing"
)

func BenchmarkCollection_Insert(b *testing.B) {
Environment(func(filename string) {
c, _ := OpenCollection(filename)
defer c.Close()
b.ResetTimer()
for i := 0; i < b.N; i++ {
c.Insert(map[string]interface{}{
"id": strconv.Itoa(i),
"hello": "world",
})
}
})
}
