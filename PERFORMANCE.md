### PERFORMANCE

Stress tests demonstrate controllable behavior and predictable resource consumption under extreme workloads.

| Files  | CREATE (Time / RAM / CPU)    | DIFF (Time / RAM / CPU)      | LIST (Time / RAM / CPU)      | Treeball Size |
|--------|------------------------------|------------------------------|------------------------------|---------------|
| 10K    | 0.04 s / 26.63 MB / 200%     | 0.04 s / 14.73 MB / 175%     | 0.03 s / 13.02 MB / 100%     | 49 KB         |
| 500K   | 0.95 s / 56.55 MB / 425%     | 1.05 s / 83.87 MB / 255%     | 0.95 s / 44.42 MB / 148%     | 2.4 MB        |
| **1M** | **1.94 s / 57.23 MB / 422%** | **1.97 s / 81.84 MB / 253%** | **1.87 s / 43.13 MB / 151%** | **4.8 MB**    |
| 5M     | 12.99 s / 62.99 MB / 317%    | 9.97 s / 82.31 MB / 252%     | 9.32 s / 47.24 MB / 151%     | 24 MB         |
| 10M    | 29.78 s / 58.88 MB / 277%    | 19.37 s / 84.13 MB / 260%    | 18.80 s / 45.23 MB / 150%    | 48 MB         |
| 50M    | 137.56 s / 61.81 MB / 309%   | 97.81 s / 142.35 MB / 256%   | 94.24 s / 74.84 MB / 145%    | 237 MB        |
| 100M   | 271.98 s / 57.02 MB / 312%   | 202.21 s / 270.82 MB / 256%  | 192.70 s / 138.55 MB / 146%  | 473 MB        |
| 150M   | 418.84 s / 54.67 MB / 303%   | 303.66 s / 400.73 MB / 267%  | 283.87 s / 204.19 MB / 152%  | 709 MB        |
| 200M   | 555.87 s / 54.88 MB / 305%   | 413.36 s / 539.45 MB / 263%  | 378.78 s / 269.55 MB / 144%  | 944 MB        |
| 250M   | 693.37 s / 53.04 MB / 305%   | 492.27 s / 658.67 MB / 265%  | 470.27 s / 334.81 MB / 152%  | 1.2 GB        |
| 300M   | 870.23 s / 53.89 MB / 292%   | 613.25 s / 788.35 MB / 254%  | 569.55 s / 397.55 MB / 150%  | 1.4 GB        |
| 400M   | 1144.05 s / 52.66 MB / 296%  | 786.33 s / 1048.24 MB / 258% | 772.39 s / 529.05 MB / 147%  | 1.9 GB        |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  
> RAM usage per million files drops significantly with scale due to **external sorting** and streaming data.  

**Benchmark Environment**:  
Average path length: ~80 characters / Maximum directory depth: 5 levels  
Default settings / `--tmpdir` (on same disk) / Maximum compression level (9)  
i5-12600K 3.69 GHz (16 cores), 32GB RAM, 980 Pro NVMe (EXT4), Ubuntu 24.04.2  
