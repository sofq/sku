# M1 Benchmark Baseline

Point-lookup performance measured with the fixture-built OpenRouter shard (5 rows, 2 USD models).

| Benchmark | ns/op (median) | Target |
|---|---|---|
| `BenchmarkPointLookup_Warm` | 66,357 (~0.07 ms) | < 5,000,000 (5 ms) |
| `BenchmarkPointLookup_Cold` | 8,737,389 (~8.7 ms) | < 60,000,000 (60 ms) |

**Go version:** go1.25.2  
**OS/arch:** darwin/arm64 (Apple M2)  
**Shard row count:** 5

Both targets met comfortably (warm is ~75x under target; cold is ~7x under target).
