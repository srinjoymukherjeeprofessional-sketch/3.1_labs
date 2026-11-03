# Assignment 3: CSMA & CSMA/CD Evaluation Statistics

## 1. Executive Summary

| MAC Protocol | Delivery Ratio (PDR) | Efficiency under Heavy Load | Collision Behavior | Key Takeaway |
| :--- | :---: | :---: | :--- | :--- |
| **CSMA/CD (BEB)** | **100.0%** | **42.6%** ($N=16$) | Collisions resolved adaptively via BEB ($2^{\min(k, 10)}$ slots) | **Superior throughput and 0% frame loss** |
| **p-Persistent ($p=0.10$)** | **26.5% - 92.0%** | **26.3%** ($N=16$) | Low $p$ defers collisions, spreading attempts over slots | **Best pure CSMA scheme** |
| **p-Persistent ($p=0.30$)** | **4.3% - 68.0%** | **8.7%** ($N=16$) | Higher transmission probability causes more overlapping slots | Moderate performance |
| **Non-Persistent** | **6.3% - 20.0%** | **11.7%** ($N=16$) | Random deferral on busy sensing avoids continuous collisions | Moderate recovery |
| **1-Persistent** | **0.0% - 4.0%** | **0.0%** ($N=16$) | All queued stations transmit simultaneously as soon as channel is idle | Severe collapse under high contention |

---

## 2. Protocol Performance Profiles

### CSMA/CD with Binary Exponential Backoff
```
═══ CSMA/CD Performance Summary (N=4 Stations, 25 frames each) ═══
  MAC Mode              : csma-cd
  Active Stations       : 4
  Propagation Delay (Tp): 5ms
  Frame Duration (Tfr)  : 10ms
  Slot Time (2*Tp)      : 10ms
  Total Attempts        : 148
  Successful Deliveries : 100 / 100
  Collision Events      : 25
  Frames Dropped        : 0
  Throughput            : 21.11 kbps
  Channel Efficiency    : 41.22%
  Delivery Ratio (PDR)  : 100.00%
══════════════════════════════════════════════════════════════════
```

### Non-Persistent CSMA
```
═══ Non-Persistent CSMA Summary (N=4 Stations, 25 frames each) ═══
  MAC Mode              : non-persistent
  Active Stations       : 4
  Propagation Delay (Tp): 5ms
  Frame Duration (Tfr)  : 10ms
  Total Attempts        : 100
  Successful Deliveries : 20 / 100
  Collision Events      : 47
  Frames Dropped        : 80
  Throughput            : 15.69 kbps
  Channel Efficiency    : 30.64%
  Delivery Ratio (PDR)  : 20.00%
══════════════════════════════════════════════════════════════════
```

### 1-Persistent CSMA
```
═══ 1-Persistent CSMA Summary (N=4 Stations, 25 frames each) ═════
  MAC Mode              : 1-persistent
  Active Stations       : 4
  Propagation Delay (Tp): 5ms
  Frame Duration (Tfr)  : 10ms
  Total Attempts        : 100
  Successful Deliveries : 4 / 100
  Collision Events      : 66
  Frames Dropped        : 96
  Throughput            : 4.94 kbps
  Channel Efficiency    : 9.65%
  Delivery Ratio (PDR)  : 4.00%
══════════════════════════════════════════════════════════════════
```

### p-Persistent CSMA ($p=0.10$)
```
═══ p-Persistent CSMA Summary (N=4 Stations, p=0.10) ═════════════
  MAC Mode              : p-persistent (p = 0.10)
  Active Stations       : 4
  Propagation Delay (Tp): 5ms
  Frame Duration (Tfr)  : 10ms
  Total Attempts        : 100
  Successful Deliveries : 76 / 100
  Collision Events      : 12
  Frames Dropped        : 24
  Throughput            : 10.30 kbps
  Channel Efficiency    : 20.11%
  Delivery Ratio (PDR)  : 76.00%
══════════════════════════════════════════════════════════════════
```

---

## 3. Generated Evaluation Plots

All high-resolution figures are saved in `eval/plots/`:
1. `eval/plots/throughput_vs_stations.png` — Throughput (kbps) vs Station Count ($N$)
2. `eval/plots/efficiency_vs_stations.png` — Channel Efficiency (%) vs Station Count ($N$)
3. `eval/plots/collisions_vs_stations.png` — Collision Count vs Station Count ($N$)
4. `eval/plots/delivery_ratio_vs_stations.png` — Packet Delivery Ratio (%) vs Station Count ($N$)
5. `eval/plots/comprehensive_comparison.png` — 2x2 comparison grid

---

## 4. p-Persistent CSMA as a Function of $p$ (Fixed $N=5$ Stations)

### Numerical Results Table

| Probability $p$ | Total Collisions | Avg Frame Delay (ms) | Throughput (kbps) | Channel Efficiency (%) | Delivery Ratio (PDR %) |
| :---: | :---: | :---: | :---: | :---: | :---: |
| **0.02** | 7 | 585.22 | 3.23 | 6.31% | 88.80% |
| **0.05** | 7 | 239.41 | 8.01 | 15.65% | 88.80% |
| **0.10** | 13 | 128.83 | 12.42 | 24.27% | 79.20% |
| **0.15** | 28 | 69.72 | 18.13 | 35.41% | 56.80% |
| **0.20** | 40 | 56.78 | 15.08 | 29.45% | 40.00% |
| **0.25** | 40 | 46.89 | **19.42 (Peak)** | **37.93%** | 40.00% |
| **0.30** | 45 | 46.33 | 13.64 | 26.64% | 30.40% |
| **0.40** | 50 | 34.77 | 14.88 | 29.07% | 25.60% |
| **0.50** | 61 | 28.58 | 9.22 | 18.02% | 12.00% |
| **0.60** | 72 | 22.66 | 6.59 | 12.88% | 7.20% |
| **0.70** | 75 | 19.86 | 4.34 | 8.47% | 4.00% |
| **0.80** | 78 | 19.09 | 3.68 | 7.18% | 3.20% |
| **0.90** | 87 | 15.20 | 2.25 | 4.40% | 1.60% |
| **1.00** | 93 | 13.39 | **0.00** | 0.00% | 0.00% |

### Theoretical Observations
1. **Collisions vs $p$:** Monotonically increases as $p \to 1.0$. At $p=0.02$, only 7 collisions occurred; at $p=1.00$, 93 collisions occurred, wiping out all successful transmissions.
2. **Transmission Delay vs $p$:** At very small $p$ ($p=0.02$), delay is highest (**585.22 ms**) because stations defer an average of $1/p = 50$ slots before attempting transmission. As $p$ increases, deferral delay drops drastically.
3. **Throughput vs $p$:** Follows the classic unimodal curve. Low at small $p$ due to excessive idle channel slots; **peaks around $p^* \approx 1/N = 1/5 = 0.20 - 0.25$**; and collapses to 0 kbps as $p \to 1.0$ due to 1-persistent-style continuous collision destruction.

### Generated Plots
* `eval/plots/p_sweep_collisions.png` — Total Collisions vs $p$
* `eval/plots/p_sweep_delay.png` — Transmission Delay (ms) vs $p$
* `eval/plots/p_sweep_throughput.png` — Throughput (kbps) vs $p$
* `eval/plots/p_sweep_combined.png` — 3-Panel Combined Comparison Plot
