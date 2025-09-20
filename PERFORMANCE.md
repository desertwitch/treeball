### PERFORMANCE

Stress tests demonstrate controllable behavior and predictable resource consumption under extreme workloads.

| Files  | CREATE  (Time / RAM / CPU)    | DIFF TAR/TAR  (Time / RAM / CPU) | DIFF TAR/FOLDER  (Time / RAM / CPU) | DIFF FOLDER/FOLDER  (Time / RAM / CPU) | LIST  (Time / RAM / CPU)      | Treeball Size |
|--------|-------------------------------|----------------------------------|-------------------------------------|----------------------------------------|-------------------------------|---------------|
| 10K    | 0.04 s / 29.44 MB / 200%      | 0.04 s / 16.58 MB / 150%         | 0.05 s / 16.53 MB / 140%            | 0.06 s / 14.02 MB / 100%               | 0.04 s / 13.53 MB / 75%       | 49 KB         |
| 500K   | 0.94 s / 55.47 MB / 435%      | 1.39 s / 88.57 MB / 243%         | 1.92 s / 77.69 MB / 150%            | 1.59 s / 96.35 MB / 133%               | 1.31 s / 45.94 MB / 140%      | 2.4 MB        |
| **1M** | **1.77 s / 58.91 MB / 469%**  | **2.44 s / 88.16 MB / 263%**     | **2.88 s / 80.04 MB / 193%**        | **2.73 s / 96.75 MB / 143%**           | **2.17 s / 46.23 MB / 141%**  | **4.8 MB**    |
| 5M     | 12.99 s / 62.83 MB / 321%     | 11.81 s / 84.08 MB / 250%        | 12.38 s / 83.80 MB / 207%           | 10.21 s / 98.65 MB / 185%              | 10.74 s / 46.04 MB / 146%     | 24 MB         |
| 10M    | 29.27 s / 59.39 MB / 291%     | 22.92 s / 86.21 MB / 256%        | 24.78 s / 79.99 MB / 212%           | 20.25 s / 113.56 MB / 181%             | 22.12 s / 46.03 MB / 140%     | 48 MB         |
| 50M    | 147.57 s / 61.06 MB / 287%    | 119.93 s / 143.62 MB / 260%      | 116.53 s / 152.81 MB / 227%         | 111.35 s / 144.43 MB / 172%            | 105.33 s / 77.20 MB / 146%    | 237 MB        |
| 100M   | 283.95 s / 54.52 MB / 298%    | 229.60 s / 278.26 MB / 256%      | 234.73 s / 273.74 MB / 222%         | 217.08 s / 276.15 MB / 177%            | 209.36 s / 143.66 MB / 146%   | 473 MB        |
| 150M   | 430.45 s / 56.27 MB / 294%    | 339.64 s / 411.65 MB / 256%      | 338.97 s / 407.49 MB / 220%         | 321.68 s / 407.44 MB / 176%            | 315.95 s / 209.40 MB / 149%   | 709 MB        |
| 200M   | 563.30 s / 55.84 MB / 300%    | 471.32 s / 564.64 MB / 262%      | 461.45 s / 541.18 MB / 225%         | 408.94 s / 541.00 MB / 187%            | 416.69 s / 276.10 MB / 147%   | 944 MB        |
| 250M   | 712.80 s / 55.27 MB / 297%    | 573.45 s / 692.66 MB / 261%      | 583.26 s / 673.68 MB / 221%         | 545.68 s / 673.45 MB / 187%            | 528.15 s / 343.45 MB / 149%   | 1.2 GB        |
| 300M   | 886.52 s / 58.55 MB / 292%    | 682.86 s / 808.64 MB / 264%      | 715.14 s / 806.09 MB / 220%         | 613.42 s / 807.07 MB / 186%            | 625.67 s / 408.66 MB / 147%   | 1.4 GB        |
| 400M   | 1147.61 s / 58.80 MB / 295%   | 932.29 s / 1074.54 MB / 258%     | 943.27 s / 1073.06 MB / 216%        | 832.93 s / 1072.90 MB / 183%           | 838.72 s / 543.02 MB / 146%   | 1.9 GB        |
| 500M   | 1434.70 s / 55.99 MB / 296%   | 1112.67 s / 1339.64 MB / 259%    | 1137.69 s / 1338.99 MB / 220%       | 1100.22 s / 1338.49 MB / 177%          | 1039.16 s / 673.95 MB / 149%  | 2.4 GB        |

> CPU usage above 100% indicates that the program is **multi-threaded** and effectively parallelized.  
> RAM usage per million files drops significantly with scale due to **external sorting** and streaming data.  

**Benchmark Environment**:  
Average path length: ~80 characters / Maximum directory depth: 5 levels  
3x `--exclude` / `--tmpdir` (on same disk) / Maximum compression level (9)  
i5-12600K 3.69 GHz (16 cores), 32GB RAM, 980 Pro NVMe (EXT4), Ubuntu 24.04.2  
