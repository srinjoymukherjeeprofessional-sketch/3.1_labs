# Assignment 4: CDMA Multiple Access Simulation

A complete simulation of **Code Division Multiple Access (CDMA)** over a shared common channel using orthogonal **Walsh Sets (Hadamard Matrices)** in Go.

---

## 📡 Architecture Overview

1. **Orthogonal Walsh Codes ($H_M$)**:
   - For $N$ stations, the chip sequence length $M = 2^{\lceil \log_2 N \rceil}$.
   - Matrix generated recursively via Sylvester's construction:
     $$H_1 = [1], \quad H_{2k} = \begin{bmatrix} H_k & H_k \\ H_k & -H_k \end{bmatrix}$$
   - Any two rows are strictly orthogonal: $W_i \cdot W_j = 0$ ($i \ne j$) and $W_i \cdot W_i = M$.

2. **Bipolar Data Modulation**:
   - Bit `1` $\to +1$
   - Bit `0` $\to -1$
   - Silent / Idle $\to 0$
   - Chip sequence for station $i$: $C_i = d_i \times W_i$.

3. **Additive Superposition Channel (Slotted Round Barrier)**:
   - Senders submit their chip sequence for the active round.
   - The Channel Server performs linear algebraic superposition:
     $$S = \sum_{i=1}^N C_i$$
   - Server broadcasts $S$ back to all stations.

4. **Reconstruction**:
   - Each station computes the normalized inner product with the target station's code $W_k$:
     $$\text{Decision} = \frac{1}{M} (S \cdot W_k) = d_k$$
   - Result:
     - $+1 \implies$ Bit 1
     - $-1 \implies$ Bit 0
     - $0 \implies$ Silent (no data)

---

## 🛠 Compilation

To build the server and station binaries:

```bash
cd ass4
mkdir -p bin
go build -o bin/server ./cmd/server
go build -o bin/station ./cmd/station
```

---

## 🚀 How to Run (Interactive Manual Multi-Terminal Mode)

### Step 1: Start the Channel Server
In Terminal 1, start the channel server expecting $N=2$ (or 4) stations:

```bash
cd ass4
./bin/server -port 8080 -n 2
```

### Step 2: Start Station 1
In Terminal 2:

```bash
cd ass4
./bin/station -server localhost:8080
```

### Step 3: Start Station 2
In Terminal 3:

```bash
cd ass4
./bin/station -server localhost:8080
```

Once all $N$ stations connect, the server begins Round 1. Each station prompts for a bit (`1`, `0`, or `s`), and once both stations enter their input, the server adds them together, broadcasts the composite signal, and both terminals reconstruct each other's data!
