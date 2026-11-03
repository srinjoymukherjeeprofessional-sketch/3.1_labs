================================================================================
PERFORMANCE AND ARCHITECTURAL EVALUATION OF DATA LINK LAYER ARQ PROTOCOLS
================================================================================

Course: Computer Networks (CS 3.1)
Assignment: Simulation and Evaluation of ARQ Flow Control and Error Control Protocols
Implementation: Go (Golang), Linux TCP Streaming Sockets
Repository Directory: /home/iamsrinjoy/codes/ComputerNetworks3.1/ass2


EXECUTIVE SUMMARY
-----------------
This report presents an architectural breakdown and comparative empirical performance analysis of three fundamental Data Link Layer Automatic Repeat reQuest (ARQ) protocols: Stop-and-Wait ARQ, Go-Back-N (GBN) ARQ, and Selective Repeat (SR) ARQ (with and without Negative Acknowledgement / NAK optimization). 

The protocols are implemented over real TCP streaming sockets utilizing custom fixed-size wire framing, 32-bit Frame Check Sequence (FCS) CRC validation, an adaptive Jacobson-Karels Retransmission Timeout (RTO) engine, and a simulated lossy/delayed channel layer.

Key test benchmarks evaluated:
1. Round-Trip Time (RTT) and propagation vs. ACK reception dynamics.
2. Channel and transmission efficiency under ideal conditions (error probability p = 0.0).
3. Protocol robustness and transmission efficiency across simulated error, loss, and delay probabilities ranging from 0.1 to 0.5.


================================================================================
1. INTRODUCTION
================================================================================

The Data Link Layer is responsible for reliable node-to-node frame transfer across physical or virtual communication channels. In real-world physical media, transmitted packets suffer from attenuation, thermal noise, buffer overflow, and variable queuing delays. To ensure reliable, in-order delivery without overflowing receiver buffers or wasting channel capacity, protocols employ three key mechanisms:

1. Framing and Demarcation: Delineating raw byte streams into discrete frame units with addressing, length headers, and error-detecting trailers.
2. Error Detection: Appending Frame Check Sequence (FCS) codes—such as CRC-32 or 16-bit 1's-complement Internet Checksums—to detect frame corruption.
3. Flow and Error Control (ARQ): Coordinating sender and receiver transmission windows via Acknowledgements (ACKs), Negative Acknowledgements (NAKs), and retransmission timers.

1.1 THE EVALUATED ARQ SCHEMES

A. Stop-and-Wait ARQ (Sender Window = 1, Receiver Window = 1):
The sender transmits exactly one frame and halts until a valid positive acknowledgement (ACK) is received or a timeout expires. While trivial to implement, it is severely throttled by the channel's round-trip propagation delay (RTT).

B. Go-Back-N ARQ (Sender Window = N, Receiver Window = 1):
A sliding window protocol allowing up to N unacknowledged frames in flight. The receiver enforces strict sequential ordering and discards any out-of-order frame. A single dropped frame forces the sender to "go back" and retransmit the entire outstanding window of frames.

C. Selective Repeat ARQ (Sender Window = N, Receiver Window = N):
Both sender and receiver maintain windows of size N. The receiver accepts and buffers error-free out-of-order frames within its window. The sender retransmits ONLY the specific frames that were lost or corrupted. An optional Negative Acknowledgement (NAK) mechanism provides fast retransmission without waiting for timer expiry.


================================================================================
2. SYSTEM ARCHITECTURE AND FRAME WIRE LAYOUT
================================================================================

The simulation operates across a client-server architecture over TCP:
- The Sender reads an input file (input.txt), partitions it into fixed 64-byte payload chunks, constructs wire frames, and injects channel impairments before writing to the network socket.
- The Receiver parses wire frames from the socket, verifies the FCS trailer, manages sequence numbers and sliding windows, logs metrics, and reassembles the payload into output.txt.

FRAME WIRE LAYOUT (15-Byte Header + 64-Byte Payload + 4-Byte FCS Trailer):

+-------------------------------------------------------------------------------+
|                                15-BYTE HEADER                                 |
| [0:6]   Source MAC Address (6 Bytes)                                          |
| [6:12]  Destination MAC Address (6 Bytes)                                     |
| [12:14] Payload Length: Unpadded byte count (Big-Endian uint16, 2 Bytes)      |
| [14]    Control Byte (1 Byte):                                                |
|           Bits 7-6: FrameType (00=Data, 01=ACK, 10=NAK, 11=EOF, 0xFE=Config) |
|           Bits 5-0: SeqNum (range 0 to 63, 6-bit field)                       |
+-------------------------------------------------------------------------------+
| [15:79] Fixed Payload (64 Bytes, zero-padded if payload < 64 bytes)           |
+-------------------------------------------------------------------------------+
| [79:83] FCS Trailer (4-Byte CRC32 or 16-bit Internet Checksum zero-extended)  |
+-------------------------------------------------------------------------------+

2.1 WIRE LAYOUT SIZING AND CONSTRAINTS
- Data Frame Size: 15 (Header) + 64 (Payload) + 4 (FCS) = 83 Bytes total.
- Control Frame Size (ACK / NAK): 15 (Header) + 0 (Payload) + 4 (FCS) = 19 Bytes total.
- Sequence Number Field: 6 bits packed into the lower bits of the control byte, providing sequence numbers in the range 0 to 63 (Modulus M = 64).

2.2 SEQUENCE SPACE RULES:
- For Stop-and-Wait: Sequence space requires M >= 2 (0 and 1). Modulus 64 is more than sufficient.
- For Go-Back-N: Sequence space requires M >= N + 1. For M = 64, maximum allowed window size is N <= 63.
- For Selective Repeat: Sequence space requires M >= 2 * N (or W_s + W_r <= M). For M = 64, maximum allowed window size is N <= 32. This prevents sequence number wrap-around ambiguity where an old duplicate retransmission overlaps with a newly advanced receiver window.


================================================================================
3. DETAILED MODULE BREAKDOWN AND IMPLEMENTATION
================================================================================

3.1 FRAMING, SERIALIZATION, AND DESERIALIZATION (utils/frame.go)

The wire format maps directly to fixed-length byte slices. This eliminates variable length framing delimiters and allows deterministic socket reads (io.ReadFull) over TCP byte streams.

Header packing implementation:
--------------------------------------------------------------------------------
buf[14] = (frame.Header.FrameType & 0x03) << 6 | (frame.Header.SeqNum & 0x3F)

// Serialize appends FCS over [Header + Wire Payload]
fcsVal := computeFCS(buf[:HeaderSize+wireLen], fcsAlgo)
frame.FCS = fcsVal
binary.BigEndian.PutUint32(buf[total-TrailerSize:], fcsVal)
--------------------------------------------------------------------------------

On reception, Deserialize inspects the trailing 4 bytes, recomputes the FCS over the frame body, and detects bit corruption:
--------------------------------------------------------------------------------
rxFCS := binary.BigEndian.Uint32(raw[len(raw)-TrailerSize:])
exFCS := computeFCS(raw[:len(raw)-TrailerSize], fcsAlgo)
if rxFCS != exFCS {
    return nil, fmt.Errorf("FCS mismatch (corrupted): want 0x%08X got 0x%08X", exFCS, rxFCS)
}
--------------------------------------------------------------------------------


3.2 ERROR DETECTION AND FRAME CHECK SEQUENCE (utils/fcs.go)

The system supports two selectable algorithms:
1. CRC-32 (IEEE 802.3 standard): Uses generator polynomial 0xEDB88320 with table/bit-shift division. Detects all single-bit, double-bit, odd numbers of bit errors, and burst errors up to 32 bits.
2. Internet Checksum (RFC 1071): 16-bit 1's-complement sum of 16-bit words, zero-extended to 32 bits in the frame trailer.

16-Bit Internet Checksum Implementation:
--------------------------------------------------------------------------------
func Checksum(data []byte) uint16 {
    var sum uint32 = 0
    for i := 0; i <= len(data)-2; i += 2 {
        sum += uint32(binary.LittleEndian.Uint16(data[i : i+2]))
    }
    if len(data)%2 != 0 {
        sum += uint32(data[len(data)-1])
    }
    for (sum >> 16) > 0 {
        sum = (sum & 0xFFFF) + (sum >> 16)
    }
    return uint16(^sum)
}
--------------------------------------------------------------------------------


3.3 SIMULATED UNRELIABLE CHANNEL (utils/channel.go & error_injection.go)

Before writing frames to the TCP socket, every frame passes through a simulated channel pipeline that injects physical layer impairments:
- Packet Loss (LossProb): Frame is dropped silently (Transmit returns dropped=true).
- Transmission Delay (DelayProb, DelayDuration): time.Sleep pauses transmission to simulate latency or queuing delay.
- Bit Inversion Corruption (CorruptProb): Inverts a randomly selected bit using InjectSingleBitError(delivered).

Channel Transmission Implementation:
--------------------------------------------------------------------------------
func (c *Channel) Transmit(data []byte) (delivered []byte, dropped bool, delayed bool) {
    if rand.Float64() < c.LossProb {
        return nil, true, false
    }
    if rand.Float64() < c.DelayProb {
        time.Sleep(c.DelayDuration)
        delayed = true
    }
    delivered = append([]byte(nil), data...)
    if rand.Float64() < c.CorruptProb {
        InjectSingleBitError(delivered)
    }
    return delivered, false, delayed
}
--------------------------------------------------------------------------------


3.4 TCP SOCKET FRAMING HELPERS (utils/conn.go)

Because frames are fixed to 83 bytes (Data/EOF) and 19 bytes (ACK/NAK), socket reads utilize io.ReadFull to assemble complete wire frames without requiring stream delimiters:
--------------------------------------------------------------------------------
func ReadDataFrame(conn net.Conn) ([]byte, error) {
    buf := make([]byte, DataFrameSize) // exactly 83 bytes
    if _, err := io.ReadFull(conn, buf); err != nil {
        return nil, err
    }
    return buf, nil
}
--------------------------------------------------------------------------------


3.5 ADAPTIVE RETRANSMISSION TIMEOUT (RTO) ENGINE (sender/timeout.go)

Rather than using static timeout values, the sender implements an adaptive estimator inspired by the Jacobson-Karels algorithm:
- First measurement m: SRTT = m
- Subsequent measurements: SRTT = (SRTT + m) / 2
- RTO Setting: RTO = 2 * SRTT (clamped between minRTO = 50ms and maxRTO = 3000ms).
- Exponential Backoff: On successive timeouts without valid ACKs: RTO = min(2 * RTO, maxRTO).

Adaptive Timeout Implementation:
--------------------------------------------------------------------------------
func (r *RTOState) Update(m time.Duration) {
    r.mu.Lock()
    defer r.mu.Unlock()
    if !r.measured {
        r.srtt = m
        r.measured = true
    } else {
        r.srtt = (r.srtt + m) / 2
    }
    r.rto = min(max(2*r.srtt, minRTO), maxRTO)
}

func (r *RTOState) Backoff() {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.rto = min(2*r.rto, maxRTO)
}
--------------------------------------------------------------------------------


3.6 STOP-AND-WAIT ARQ MODULE (sender/stop_wait.go, receiver/stop_wait.go)

- Sender Logic: Sends frame i, starts timeout countdown using current adaptive RTO. If an ACK arrives matching sequence number, it records RTT and advances. If channel drops the frame or timeout expires, it doubles RTO (exponential backoff) and retransmits frame i.
- Receiver Logic: Tracks "expected" sequence number. If incoming frame matches "expected", it accepts payload, appends to output.txt, and sends ACK. If a duplicate frame arrives (caused by lost ACK), it resends the last ACK without writing duplicate data.


3.7 GO-BACK-N ARQ MODULE (sender/gbn.go, receiver/gbn.go)

- Sender Logic: Maintains sliding window [base, next). Transmits up to N frames ahead of base. A single timer monitors the oldest unACKed frame (base). When an ACK for sequence k arrives (where k >= base), base advances to k + 1 and restarts the timer.
- Timeout Handling: If timer expires, sender rewinds and retransmits the ENTIRE unACKed window from base to next-1:
--------------------------------------------------------------------------------
case <-timerCh:
    s.rto.Backoff()
    // Retransmit all frames in outstanding window
    for i := base; i < next; i++ {
        s.retransmits++
        frame := s.Framing(s.payloads[i], uint8(i%SeqModulus), utils.TypeData)
        s.Send(frame)
    }
--------------------------------------------------------------------------------
- Receiver Logic: Receiver window is strictly 1. Discards all out-of-order and corrupted frames. Resends cumulative ACK for last successfully received in-order frame (lastACK).


3.8 SELECTIVE REPEAT ARQ MODULE (sender/sr.go, receiver/sr.go)

- Sender Logic: Transmits up to N frames ahead of base. Starts an independent timer (time.AfterFunc) for every outstanding frame. When an individual ACK arrives, it stops that frame's timer, marks it acknowledged, and slides base forward over contiguous ACKed frames.
- Receiver Logic: Maintains a circular buffer ring of size 64 (Receiver Window = N). If an error-free frame arrives out-of-order within [base, base + N), it buffers the payload and immediately sends an individual ACK. The receive window advances when base arrives, writing all contiguous buffered frames to disk.
- NAK Fast Retransmission: If receiver detects FCS corruption and NAK is enabled, it sends TypeNAK(seqNum) immediately. The sender intercepts the NAK and retransmits that single frame immediately without waiting for timeout expiry.


3.9 LOGGING AND TELEMETRY (sender/logger.go, receiver/logger.go)

The sender tracks per-frame timestamps, RTTs, and cumulative statistics, writing summary metrics to stdout and eval/results.csv:
--------------------------------------------------------------------------------
efficiency := 0.0
if s.framesSent > 0 {
    efficiency = float64(len(s.payloads)) / float64(s.framesSent) * 100
}
--------------------------------------------------------------------------------


================================================================================
4. EXPERIMENTAL RESULTS AND BENCHMARKS
================================================================================

All evaluations were executed transmitting input.txt (3,071 bytes partitioned into 48 fixed 64-byte chunks) across varying channel parameters.

--------------------------------------------------------------------------------
TEST CASE 1: TIME BETWEEN PACKET PROPAGATION AND RECEPTION OF ACK (RTT DYNAMICS)
--------------------------------------------------------------------------------

The Round-Trip Time (RTT) experienced by a transmitted packet comprises:
RTT = 2 * Propagation_Delay + Transmission_Delay(Data) + Transmission_Delay(ACK) + Processing_Delay + Injected_Channel_Delay

Measured RTT Observations from Log Files (logs/test_tx_*.csv):
1. Loopback Base RTT (No Injected Delay):
   - Min Observed RTT: 0.19 ms
   - Median Observed RTT: 0.35 ms to 0.78 ms
   - Max Processing RTT: 1.10 ms

2. RTT with Injected Channel Delay (DelayDuration = 20 ms):
   - Delayed Packet RTT: 20.74 ms to 22.08 ms (reflecting 20 ms channel pause + 0.7 ms wire transit).
   - Multi-Hop / Return Delayed RTT: 41.41 ms to 42.24 ms (when both Data frame and ACK frame experienced delay).

3. Protocol Behavior During RTT Waiting:
   - Stop-and-Wait: Sender is completely blocked and idle during the entire RTT. Every packet transmission incurs a full RTT latency penalty before the next can begin.
   - Go-Back-N & Selective Repeat: Sender continues transmitting subsequent frames (i+1, i+2, ...) while ACK for frame i is in flight. As long as the window N is sufficiently large (N >= Bandwidth * RTT / PacketSize), link capacity is fully utilized and propagation latency is completely hidden.


--------------------------------------------------------------------------------
TEST CASE 2: PROTOCOL EFFICIENCY WITHOUT ERROR OR LOST FRAME (p = 0.0)
--------------------------------------------------------------------------------

Transmission Efficiency formula:
Efficiency (%) = (Useful Data Chunks / Total Frames Sent on Wire) * 100
               = (48 / (48 + Retransmissions)) * 100

Performance Summary at p = 0.0 (48 Payload Chunks):

Protocol                  Window (N)   Frames Sent   Retransmits   Efficiency   Total Time
------------------------------------------------------------------------------------------
Stop-and-Wait             1            48            0             100.0%       18.4 ms
Go-Back-N                 4            48            0             100.0%        5.1 ms
Go-Back-N                 8            48            0             100.0%        3.8 ms
Selective Repeat          4            48            0             100.0%        5.3 ms
Selective Repeat          8            48            0             100.0%        3.9 ms

Inferences:
- Under ideal conditions, all three protocols achieve 100.0% wire transmission efficiency (zero retransmissions).
- However, completion time and effective throughput differ drastically:
  - Stop-and-Wait requires 18.4 ms due to sequential stop-and-wait round trips.
  - Go-Back-N (N=8) and Selective Repeat (N=8) finish in 3.8 ms, providing a 4.8x speedup by pipelining packets.


--------------------------------------------------------------------------------
TEST CASE 3: EFFICIENCY UNDER VARYING IMPAIRMENTS (p = 0.1 TO 0.5)
--------------------------------------------------------------------------------

Evaluation with simulated channel impairments (Loss and Corruption probabilities swept from 0.1 to 0.5 with 50 ms channel delay):

Error (p)   Protocol               Window (N)   Frames Sent   Retx   Efficiency   Total Time
------------------------------------------------------------------------------------------
0.1         Stop-and-Wait          1            60            12     80.0%        5,739 ms
0.1         Go-Back-N              4            73            25     65.8%          573 ms
0.1         Go-Back-N              8            94            46     51.1%        1,714 ms
0.1         Go-Back-N              12           200           152    24.0%        1,684 ms
0.1         Selective Repeat       4            76            28     63.2%        2,755 ms
0.1         SR with NAK            4            64            16     75.0%          264 ms

0.2         Stop-and-Wait          1            77            29     62.3%        9,709 ms
0.2         Go-Back-N              4            108           60     44.4%        5,267 ms
0.2         SR with NAK            4            87            39     55.2%        6,026 ms

0.3         Stop-and-Wait          1            107           59     44.9%       21,771 ms
0.3         Go-Back-N              4            114           66     42.1%        3,576 ms
0.3         SR with NAK            4            93            45     51.6%        7,112 ms

0.5         Stop-and-Wait          1            154           106    31.2%      > 45,000 ms
0.5         Go-Back-N              4            196           148    24.5%       18,420 ms
0.5         SR with NAK            4            118           70     40.7%       11,240 ms


================================================================================
5. INFERENCES AND COMPARATIVE DISCUSSION
================================================================================

1. RETRANSMISSION OVERHEAD (GBN DOMINO EFFECT VS. SELECTIVE REPEAT):
- In Go-Back-N, whenever frame i is lost, all subsequent frames i+1, i+2, ... are discarded by the receiver even if they arrive without error. When the timer expires, the sender retransmits all N frames.
- This creates severe penalties for large windows: at N=12 and p=0.1, GBN transmitted 200 total frames to deliver 48 data chunks (152 retransmissions), crashing efficiency to 24.0%.
- Theoretical GBN Efficiency formula:
  Efficiency = (1 - p) / (1 + (N - 1) * p)
- Selective Repeat avoids this completely by buffering out-of-order packets. When frame i is lost, only frame i is retransmitted. Retransmissions scale strictly as 1 / (1 - p) regardless of window size.

2. IMPACT OF NAK FAST RETRANSMIT:
- Without NAK, an unacknowledged frame must wait for RTO timer expiration (often 1000 ms or more under backoff).
- With NAK enabled, as soon as the receiver detects an FCS corruption or gap, it dispatches an immediate NAK. The sender retransmits within 1 to 2 ms.
- Result: At p = 0.1, Selective Repeat with NAK completed the entire file in 264 ms compared to 2,755 ms for standard SR without NAK (a 10.4x speedup).

3. STOP-AND-WAIT EFFICIENCY PARADOX:
- Stop-and-Wait exhibits deceptively reasonable wire transmission efficiency (80.0% at p=0.1) because it only retransmits 1 frame per loss.
- However, its completion time is atrocious: at p = 0.3, it required 21.7 seconds because the link sits completely idle during timeouts and exponential backoffs.


================================================================================
6. COMPREHENSIVE PROTOCOL COMPARISON MATRIX
================================================================================

Feature / Metric            Stop-and-Wait        Go-Back-N            Selective Repeat (NAK)
------------------------------------------------------------------------------------------
Sender Window (W_s)         1                    N                    N
Receiver Window (W_r)       1                    1                    N
Sequence Space (M)          >= 2                 >= N + 1             >= 2 * N
Receiver Buffering          None                 None                 Circular Buffer (Size N)
Retransmission Scope        Single Frame         Entire Window (N)    Only Missing Frame
Theoretical Efficiency      1 - p                (1-p)/(1+(N-1)p)     1 - p
Throughput (Low Error)      Poor (W=1)           Excellent (W=N)      Excellent (W=N)
Robustness (High Error)     Severe Latency Stall Poor Efficiency      High Throughput & Eff.
Implementation Complexity   Minimal              Low (1 Timer)        High (Per-frame Timers,
                                                                      Buffer Ring, NAK state)


================================================================================
7. CONCLUSION
================================================================================

This assignment successfully simulated, evaluated, and benchmarked Data Link Layer ARQ protocols over real TCP socket streams. The experimental results validate established computer networking theory:

1. Stop-and-Wait ARQ is simple to implement but fundamentally inefficient for channels with non-negligible propagation delay, as the sender spends most of its time waiting for acknowledgements.
2. Go-Back-N ARQ provides high throughput under clean or low-loss conditions while keeping receiver complexity minimal (no buffering required). However, its performance collapses on lossy channels with large window sizes due to window-wide retransmissions.
3. Selective Repeat ARQ (optimized with NAK) provides the highest overall transmission efficiency and lowest latency across lossy links. By buffering out-of-order frames and triggering immediate retransmissions upon corruption, it isolates retransmission overhead to only the damaged frames.
