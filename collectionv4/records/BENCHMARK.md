# Choosing the Right Implementation for InceptionDB

InceptionDB offers several internal implementations (`Correct`, `Fast`, `Turbo`, `Ultra`, `Hyper`) with very different performance characteristics.  
Here is a practical guide based on real benchmarks (millions of operations/second on a modern multi-core CPU).

| Workload / Use Case                              | Recommended Implementation | Expected Peak Throughput | Why this one?                                                                 |
|--------------------------------------------------|----------------------------|---------------------------|-------------------------------------------------------------------------------|
| Typical REST / GraphQL APIs (balanced CRUD)      | **Ultra**                  | 20–25 M/s                 | Best scaling with high concurrency (64–128 threads), wins in mixed read/write/delete workloads |
| High write throughput (mostly inserts, append-only) | **Hyper**               | 19–20 M/s                 | Highest & most stable insert performance, ideal for logging, events, telemetry, ingestion pipelines |
| Balanced performance + simpler / lighter code    | **Turbo**                  | 15–20 M/s                 | Very consistent across concurrency levels, good compromise between speed and code complexity |
| Early prototyping, low traffic, or minimalism    | **Fast**                   | 7–18 M/s                  | Much faster than reference, still reasonably simple internally                     |
| Reference / correctness testing / education      | **Correct**                | 3–8 M/s                   | Slowest – used mainly to verify logic, not for production                          |

### Quick Decision Tree

- **Mostly inserts** (logging, metrics, event streaming, bulk ingestion)?  
  → **Hyper**

- **Mixed read/write operations** with **high concurrency** (web APIs, mobile backends, real-time services)?  
  → **Ultra**

- Want **good speed** but prefer **simpler / more predictable code** and lower resource usage?  
  → **Turbo**

- Just experimenting or running on very low-spec hardware?  
  → **Fast** (or even `Correct` for debugging)

Ultra currently holds the absolute record in mixed workloads (~25.6 M/s @ 128 threads), while Hyper leads pure writes (~19.9 M/s).

Choose based on your **real bottleneck** (use monitoring / profiling), not just the peak number.