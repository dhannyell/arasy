## Comparison: bench-runtime-2026-07-03-135320.md → bench-runtime-2026-07-03-150710.md

Negative % = faster/smaller (improvement). Positive % = slower/bigger (regression).

| Benchmark | ns/op (old → new) | Δ ns/op | B/op (old → new) | Δ B/op | allocs/op (old → new) | Δ allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `BenchmarkDriverStep/entities=100/history=0-16` | 85008.4 → 87158.8 | +2.5% | 25165.8 → 25179.2 | +0.1% | 243 → 243 | +0.0% |
| `BenchmarkDriverStep/entities=100/history=600-16` | 101371 → 95908.8 | -5.4% | 44966.2 → 44981.2 | +0.0% | 282 → 282 | -0.1% |
| `BenchmarkDriverStep/entities=1000/history=0-16` | 1176583 → 1212021.8 | +3.0% | 261923 → 261942.4 | +0.0% | 919 → 918 | -0.0% |
| `BenchmarkDriverStep/entities=1000/history=600-16` | 1313725.8 → 1494430.6 | +13.8% | 532177.2 → 532183 | +0.0% | 965 → 965 | +0.0% |
| `BenchmarkDriverBack/entities=100-16` | 33087.4 → 34894 | +5.5% | 1712 → 1712 | +0.0% | 17 → 17 | +0.0% |
| `BenchmarkDriverBack/entities=1000-16` | 361905.6 → 401037.6 | +10.8% | 16432 → 16432 | +0.0% | 17 → 17 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-1-16` | 134263.4 → 154935.2 | +15.4% | 85156.4 → 85185.2 | +0.0% | 462 → 462 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-half-16` | 29237444.8 → 34273815 | +17.2% | 13832761 → 13837723.2 | +0.0% | 91314 → 91314 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=100/timeline=600/to-last-16` | 58115541.2 → 69747029.8 | +20.0% | 27506256.2 → 27516383 | +0.0% | 179580 → 179581 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-1-16` | 147948.6 → 169745 | +14.7% | 110912.6 → 110943.6 | +0.0% | 662 → 662 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-half-16` | 174432029.8 → 196933170 | +12.9% | 82233817.2 → 82261632.8 | +0.0% | 532007 → 532006 | -0.0% |
| `BenchmarkDriverScrubToTick/entities=100/timeline=3600/to-last-16` | 362846206.6 → 370861580 | +2.2% | 164061025.8 → 164117812.6 | +0.0% | 1057440 → 1057439 | -0.0% |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-1-16` | 1779007.8 → 1945242.6 | +9.3% | 1051948.2 → 1051930.2 | -0.0% | 1155 → 1155 | -0.0% |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-half-16` | 410113813.4 → 425284200 | +3.7% | 160143694.6 → 160148975.2 | +0.0% | 288161 → 288161 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=600/to-last-16` | 817715850 → 831586230 | +1.7% | 319495399.2 → 319504579.2 | +0.0% | 580048 → 580047 | -0.0% |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-1-16` | 1887897.6 → 2038010 | +8.0% | 1077558.2 → 1077582.2 | +0.0% | 1355 → 1355 | +0.0% |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-half-16` | 2531158940 → 3027475600 | +19.6% | 959374537.6 → 959397384 | +0.0% | 1752000 → 1751998 | -0.0% |
| `BenchmarkDriverScrubToTick/entities=1000/timeline=3600/to-last-16` | 5627223220 → 5777383260 | +2.7% | 1917739219.2 → 1917803052.8 | +0.0% | 3512836 → 3512838 | +0.0% |
| `BenchmarkReplay/entities=100/ticks=600-16` | 54033286.8 → 60054583.6 | +11.1% | 15333713.8 → 15343571.4 | +0.1% | 154029 → 154029 | +0.0% |
| `BenchmarkReplay/entities=1000/ticks=600-16` | 726174050 → 857961600 | +18.1% | 157615192.8 → 157633440.8 | +0.0% | 550610 → 550623 | +0.0% |
