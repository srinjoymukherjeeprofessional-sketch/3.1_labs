# Assignment 3: CSMA & CSMA/CD Simulation (IEEE 802.3)

A high-fidelity simulation of **Carrier Sense Multiple Access (CSMA)** and **CSMA with Collision Detection (CSMA/CD)** implemented in Go over TCP sockets.

---

## 🚀 Quick Start: 3 Ways to Use

### Method 1: All-in-One Simulation Runner (Recommended)

Run a complete multi-station simulation in a single command using the built-in orchestrator:

```bash
cd ass3

# 1. Run CSMA/CD with 4 stations, 20 frames each:
./bin/sim -mode csma-cd -stations 4 -frames 20

# 2. Run Non-Persistent CSMA:
./bin/sim -mode non-persistent -stations 4 -frames 20

# 3. Run 1-Persistent CSMA:
./bin/sim -mode 1-persistent -stations 4 -frames 20

# 4. Run p-Persistent CSMA with p = 0.3:
./bin/sim -mode p-persistent -p 0.3 -stations 4 -frames 20
```

#### Orchestrator Flags:
| Flag | Default | Description |
| :--- | :--- | :--- |
| `-mode` | `csma-cd` | MAC algorithm: `csma-cd`, `non-persistent`, `1-persistent`, `p-persistent` |
| `-stations` | `4` | Number of concurrent contending stations |
| `-frames` | `30` | Number of frames each station attempts to send |
| `-tp` | `5ms` | One-way channel propagation delay ($T_p$) |
| `-tfr` | `10ms` | Frame transmission duration ($T_{fr} \ge 2 T_p$) |
| `-p` | `0.5` | Probability parameter $p$ (for `p-persistent` mode) |
| `-quiet` | `false` | Suppress per-frame terminal logs (shows summary only) |
| `-csv` | `eval/results.csv` | Output CSV file to append summary metrics |

---

### Method 2: Automated Batch Evaluation & Plotting

Run the automated test suite across all station counts ($N = 2, 4, 6, 8, 12, 16$) and all 4 MAC schemes:

```bash
cd ass3
./eval/run_eval.sh
```

This runs 30 parameter configurations, populates `eval/results.csv`, and automatically creates publication-quality plots in `eval/plots/`:
* `eval/plots/throughput_vs_stations.png`
* `eval/plots/efficiency_vs_stations.png`
* `eval/plots/collisions_vs_stations.png`
* `eval/plots/delivery_ratio_vs_stations.png`
* `eval/plots/comprehensive_comparison.png` (2x2 comparison grid)

To re-generate plots from the CSV at any time:
```bash
python3 eval/plot.py eval/results.csv eval/plots
```

---

### Method 3: p-Persistent CSMA Sweep as a Function of p (Fixed N)

To evaluate **Number of Collisions**, **Transmission Delay**, and **Throughput** as functions of probability $p$ (with a fixed number of stations $N$, e.g., $N=5$):

```bash
cd ass3
./eval/run_p_sweep.sh 5 25
```
*(Arguments: `<stations_N>` (default 5), `<frames_per_station>` (default 25))*

This generates:
* `eval/p_sweep_results.csv`
* `eval/plots/p_sweep_collisions.png` (Collisions vs $p$)
* `eval/plots/p_sweep_delay.png` (Transmission Delay vs $p$)
* `eval/plots/p_sweep_throughput.png` (Throughput vs $p$)
* `eval/plots/p_sweep_combined.png` (3-panel publication comparison)

---

### Method 4: Manual Multi-Terminal Execution (Distributed)

You can run the Channel Server and individual Stations in separate terminal windows:

#### Terminal 1: Start the Channel Server
```bash
cd ass3
./bin/server -port 8080 -tp 5ms
```

#### Terminal 2: Start Station 1
```bash
cd ass3
./bin/station -id 1 -server localhost:8080 -mode csma-cd -frames 20
```

#### Terminal 3: Start Station 2
```bash
cd ass3
./bin/station -id 2 -server localhost:8080 -mode csma-cd -frames 20
```

---

## 🛠 Rebuilding Binaries

If you modify the Go source code, recompile all binaries with:
```bash
cd ass3
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/station ./cmd/station
go build -o bin/sim ./cmd/sim
```
