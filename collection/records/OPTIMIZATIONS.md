# High-Performance Go Optimizations

This document explains the advanced optimization techniques used in `RecordsUltra` and `RecordsHyper` implementations. It details the reasoning behind each technique, its pros and cons, and when to apply it.

---

## 1. Sharding (Lock Striping)

### What is it?
Instead of having a single global data structure protected by a single lock (e.g., a single `sync.RWMutex`), the data is partitioned into multiple smaller, independent "shards". Each shard has its own dedicated lock and manages a subset of the data. 

In `RecordsUltra`, we divide the records into 512 independent shards. When inserting or reading a record, we route the operation to a specific shard. 

### Example
**Traditional (Global Lock):**
```go
type GlobalRecords struct {
    sync.Mutex
    data []string
}
// All 100 concurrent goroutines will block waiting for this single mutex.
```

**Sharded (Lock Striping):**
```go
type Shard struct {
    sync.Mutex
    data []string
}
type ShardedRecords struct {
    shards [256]*Shard
}
// 100 concurrent goroutines will likely hit different shards. 
// Very few will actually collide and block each other.
```

### When to use
- High concurrency scenarios where many threads/goroutines are frequently reading and writing to a shared data structure.
- When CPU profiling shows significant time spent in `sync.(*Mutex).Lock` or `runtime.semacquire`.

### When NOT to use
- Single-threaded applications or scenarios with low contention.
- When operations require atomicity *across* multiple shards (e.g., transferring money between two bank accounts stored in different shards). Sharding makes multi-item transactions incredibly complex and prone to deadlocks.

### Pros/Cons
- ✅ **Pros:** Drastically reduces lock contention. Scales almost linearly with the number of CPU cores.
- ❌ **Cons:** Increases memory footprint (due to multiple locks and struct overhead). Makes cross-shard operations complex and slow.

---

## 2. Segmented Arrays (Chunking)

### What is it?
Go's built-in `append()` on a standard slice (`[]T`) requires allocating a brand new, larger block of memory and copying all existing elements into it whenever the underlying array runs out of capacity. This causes massive latency spikes (O(N) copy) and GC pressure under heavy load.

A **Segmented Array** (`[][]T`) is an array of fixed-size arrays (chunks). Instead of growing a single massive contiguous block of memory, you only allocate a new small chunk when the current one is full. Elements are never moved once inserted.

### Example
**Traditional Slice:**
```go
data := make([]int, 0, 1000)
// Once 1000 items are inserted, inserting the 1001st item 
// forces the runtime to allocate ~2000 slots and copy 1000 integers.
```

**Segmented Array:**
```go
const segmentSize = 4096
var segments [][]int

// To insert:
if currentSegmentIsFull {
    segments = append(segments, make([]int, segmentSize)) // O(1) allocation
    // No existing elements are copied!
}
```

### When to use
- Building massive append-only logs, event stores, or huge in-memory databases.
- When latency predictability is critical, and garbage collection (GC) stalls caused by large slice reallocations are unacceptable.

### When NOT to use
- When the data structure is small or strictly bounded.
- If you need strict contiguous memory layout for CGO interoperability or specific SIMD instructions across the entire dataset.

### Pros/Cons
- ✅ **Pros:** Guarantees O(1) append latency. Zero memory copying of existing elements. Highly predictable GC behavior.
- ❌ **Cons:** Slower sequential range iteration (CPU cache prefetcher struggles slightly more jumping between segments). Math overhead to calculate the segment index and offset.

---

## 3. Cache-Line Padding (Preventing False Sharing)

### What is it?
Modern CPUs read and write memory in chunks called "Cache Lines" (typically 64 bytes). If two independent variables sit next to each other in memory (sharing the same cache line), and Core 1 modifies Variable A while Core 2 modifies Variable B, the CPU hardware will invalidate the entire cache line for both cores. This forces both cores to fetch data from the slow Main Memory (RAM), destroying performance. This phenomenon is called **False Sharing**.

We prevent this by inserting "padding" (dummy unused bytes) between independent structs, forcing them into separate 64-byte cache lines.

### Example
**Vulnerable to False Sharing:**
```go
// Both counters fit into a single 64-byte cache line.
type Counters struct {
    Core1Count atomic.Int64 // 8 bytes
    Core2Count atomic.Int64 // 8 bytes
}
```

**Padded (Safe):**
```go
type Counters struct {
    Core1Count atomic.Int64
    _          [56]byte // Pad to 64 bytes
    Core2Count atomic.Int64
    _          [56]byte // Pad to 64 bytes
}
```

### When to use
- When building highly concurrent data structures (like our Shards) where independent CPU cores write to independent fields frequently.
- High-frequency atomic counters mapped to different threads.

### When NOT to use
- General application code where data is mostly read, or modified by a single thread.
- When memory space is severely constrained (e.g., embedded devices), as padding wastes RAM.

### Pros/Cons
- ✅ **Pros:** Eliminates silent hardware-level CPU cache invalidations. Massive speedup in concurrent write-heavy loops.
- ❌ **Cons:** Wastes memory (56 to 64 bytes per padded variable). Code looks unusual to junior developers.

---

## 4. Bitwise Operations for Routing

### What is it?
CPUs perform operations like Addition (`+`), Subtraction (`-`), and Bitwise logic (`&`, `|`, `>>`, `<<`) extremely fast (often in 1 clock cycle). However, Division (`/`) and Modulo (`%`) are notoriously slow, taking dozens of clock cycles. 

When your bounds (like the number of shards or the segment sizes) are exact powers of 2 (e.g., 256, 1024, 4096), you can replace slow Modulo/Division with incredibly fast Bitwise operations.

### Example
Assume `numShards = 256` (which is $2^8$). The mask is `256 - 1 = 255` (binary `11111111`).

**Slow (Modulo/Division):**
```go
shardIndex := id % 256
segmentIndex := id / 4096
```

**Fast (Bitwise):**
```go
shardIndex := id & 255      // Equivalent to id % 256
segmentIndex := id >> 12    // Equivalent to id / 4096 (since 2^12 = 4096)
```

### When to use
- In the absolute hottest, most frequently executed paths of your code (like routing an ID to a database shard).
- When configuring constants for array dimensions or pool sizes—always prefer powers of 2.

### When NOT to use
- When the divisor is not guaranteed to be a power of 2. Bitwise tricks only work for $2^N$ boundaries.
- General business logic where readability is more important than raw nanosecond performance.

### Pros/Cons
- ✅ **Pros:** Free CPU cycles. Mathematically perfect routing with zero penalty.
- ❌ **Cons:** Restricts your tuning options (e.g., you can have 256 or 512 shards, but not 300 shards).

---

## 5. OS Mutexes vs. Spin-Locks

### What is it?
A standard `sync.Mutex` interacts with the Operating System (via Futexes) when it encounters contention. It pauses the current thread, puts it to sleep, and wakes it up later. This "context switch" takes several microseconds.

A **Spin-Lock**, on the other hand, never goes to sleep. It uses an `atomic.CompareAndSwap` loop to obsessively check if the lock is free. It literally "spins" the CPU core at 100% usage waiting for the lock.

### Example
**OS Mutex:**
```go
var mu sync.Mutex
mu.Lock() // If locked, puts thread to sleep (slow but saves CPU)
// do work
mu.Unlock()
```

**Spin-Lock:**
```go
var state atomic.Uint32
func Lock() {
    for !state.CompareAndSwap(0, 1) {
        runtime.Gosched() // Yield back to scheduler, but keep CPU spinning
    }
}
```

### When to use Spin-Locks
- Bare-metal environments or C/C++ systems.
- When the critical section is incredibly small (e.g., 5-10 CPU instructions) and you know the thread holding the lock will release it in nanoseconds, making the cost of an OS context switch much worse than the cost of burning CPU cycles.

### When to use OS Mutexes (`sync.RWMutex`)
- Almost always in Go. The Go scheduler handles `sync.Mutex` incredibly well by cooperatively suspending goroutines rather than blocking underlying OS threads.
- When critical sections are long or involve I/O.
- **As discovered in our benchmarks**, in highly saturated systems where there are more Goroutines than physical CPU cores, Spin-Locks can aggressively "starve" the scheduler, leading to worse performance. `sync.Mutex` efficiently orchestrates waiting goroutines, yielding better overall throughput.

### Pros/Cons
- ✅ **Pros (Spin-Lock):** Zero context-switching overhead. Unbeatable latency on lightly-loaded but highly-contended minimal sections.
- ❌ **Cons (Spin-Lock):** Burns 100% CPU while waiting. Horrendous performance degradation under massive contention (Scheduler Starvation). Not recommended for generic Go applications.
