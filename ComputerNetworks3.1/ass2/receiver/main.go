// receiver/main.go
//
// Receiver program for Assignment 2 — Data Link Layer Simulation.
//
// ─── Overview ────────────────────────────────────────────────────────────────
// The Receiver listens on a TCP port, accepts one connection from the Sender,
// reads incoming frames, checks their integrity (FCS), and writes valid payloads
// to an output file in order.
//
// Three flow-control protocols are supported, matching the Sender:
//   stop-wait  Stop-and-Wait ARQ
//   gbn        Go-Back-N ARQ (window size 1 on receiver side)
//   sr         Selective Repeat ARQ (window size N, receiver buffers out-of-order)
//
// ─── Error Handling ──────────────────────────────────────────────────────────
// For every received frame the Receiver recomputes the FCS and compares it with
// the trailer value (Check method). A mismatch means the frame was corrupted:
//   - Stop-Wait / GBN: silently discard (sender will retransmit on timeout)
//   - SR without NAK:  silently discard + buffer out-of-order frames
//   - SR with NAK:     send NAK immediately → reduces sender's retransmit latency
//
// ─── ACK Channel Simulation ──────────────────────────────────────────────────
// ACK frames sent back to the sender also pass through a channel simulation so
// that ACK loss / corruption is also modelled (matching the sender's channel).
//
// ─── Configuration ───────────────────────────────────────────────────────────
// Same as sender: config file via os.Args[1] or interactive prompts.
package main

import (
	"ass2/utils"
	"bufio"
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const SeqModulus = 64 // 6-bit sequence number space

// Hardcoded MAC addresses (must match sender)
var (
	SenderMAC   = [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	ReceiverMAC = [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x02}
)

// ─── Configuration ────────────────────────────────────────────────────────────

// Config holds all runtime parameters for the Receiver.
type Config struct {
	ListenAddr  string        // TCP address to listen on e.g. ":9000"
	Mode        string        // "stop-wait" | "gbn" | "sr"
	FCSAlgo     string        // "crc32" | "checksum" (must match sender)
	WindowSize  int           // N for SR (GBN receiver window is always 1)
	NAKEnabled  bool          // SR only: send NAK on corruption/gap detection
	LossProb    float64       // ACK channel loss probability
	DelayProb   float64       // ACK channel delay probability
	DelayDur    time.Duration // ACK channel delay duration
	CorruptProb float64       // ACK channel corruption probability
	OutputFile  string        // Path to write reassembled data
	LogPrefix   string        // Prefix for output CSV logs
}

func promptConfig() Config {
	ask := func(prompt, def string) string {
		fmt.Printf("%s [%s]: ", prompt, def)
		sc := bufio.NewScanner(os.Stdin)
		sc.Scan()
		v := strings.TrimSpace(sc.Text())
		if v == "" {
			return def
		}
		return v
	}
	parseInt := func(s string) int { v, _ := strconv.Atoi(s); return v }

	cfg := Config{}
	cfg.ListenAddr = ask("Listen address (:port)", ":9000")
	cfg.Mode = ask("Mode (stop-wait / gbn / sr)", "stop-wait")
	cfg.FCSAlgo = ask("FCS algorithm (crc32 / checksum)", "crc32")
	cfg.OutputFile = ask("Output file path", "output.txt")
	cfg.LogPrefix = ask("Log CSV prefix", "receiver")

	if cfg.Mode == "sr" {
		cfg.WindowSize = parseInt(ask("Window size N", "4"))
		cfg.NAKEnabled = ask("Enable NAK? (y/n)", "n") == "y"
	}

	// Channel impairment parameters (loss, delay, corrupt) are automatically
	// reused from the sender upon TCP connection.
	cfg.DelayDur = 100 * time.Millisecond
	return cfg
}

func loadConfig(path string) Config {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open config %s: %v\n", path, err)
		os.Exit(1)
	}
	defer f.Close()

	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config: %v\n", err)
	}
	pi := func(k string, def int) int {
		if v, err := strconv.Atoi(m[k]); err == nil && m[k] != "" {
			return v
		}
		return def
	}
	pf := func(k string, def float64) float64 {
		if v, err := strconv.ParseFloat(m[k], 64); err == nil && m[k] != "" {
			return v
		}
		return def
	}
	pb := func(k string) bool { return m[k] == "true" || m[k] == "y" }
	str := func(k, def string) string {
		if v := m[k]; v != "" {
			return v
		}
		return def
	}

	durMs := pi("delay_dur_ms", pi("ack_delay_dur_ms", 100))
	return Config{
		ListenAddr:  str("listen", ":9000"),
		Mode:        str("mode", "stop-wait"),
		FCSAlgo:     str("fcs", "crc32"),
		WindowSize:  pi("window", 4),
		NAKEnabled:  pb("nak"),
		LossProb:    pf("loss", pf("ack_loss", 0)),
		DelayProb:   pf("delay", pf("ack_delay", 0)),
		DelayDur:    time.Duration(durMs) * time.Millisecond,
		CorruptProb: pf("corrupt", pf("ack_corrupt", 0)),
		OutputFile:  str("output", "output.txt"),
		LogPrefix:   str("log_prefix", "receiver"),
	}
}

// ─── Receiver struct ──────────────────────────────────────────────────────────

// Receiver encapsulates all state for the receiving side.
type Receiver struct {
	cfg       Config
	conn      net.Conn
	ch        *utils.Channel // Channel simulation applied to outgoing ACK frames
	outFile   *os.File
	csvW      *csv.Writer
	startTime time.Time
	// Stats
	accepted   int
	discarded  int
	naksSent   int
}

// ─── Receiver methods ─────────────────────────────────────────────────────────

// Recv reads one raw frame from the TCP connection and deserialises it.
// Returns (nil, nil) if the frame is corrupted (FCS mismatch) — caller decides
// whether to send a NAK.
func (r *Receiver) Recv() (*utils.Frame, []byte, error) {
	raw, err := utils.ReadDataFrame(r.conn)
	if err != nil {
		return nil, nil, err
	}
	frame, err := utils.Deserialize(raw, r.cfg.FCSAlgo)
	if err != nil {
		// Corrupted frame — return raw bytes so caller can extract seqNum from
		// the control byte if NAK is needed (control byte itself may be intact)
		return nil, raw, nil
	}
	return frame, raw, nil
}

// Check recomputes the FCS of raw frame bytes and returns true if they match.
// (Wraps the FCS verification already done in Deserialize; exposed for clarity.)
func (r *Receiver) Check(frame *utils.Frame) bool {
	return frame != nil
}

// Send serialises an ACK/NAK frame, applies the ACK channel simulation, and
// writes the result to the TCP connection (unless the channel drops it).
func (r *Receiver) Send(frame *utils.Frame) error {
	raw, err := frame.Serialize(r.cfg.FCSAlgo)
	if err != nil {
		return err
	}
	delivered, dropped, _ := r.ch.Transmit(raw)
	if dropped {
		return nil // ACK silently lost — sender will timeout and retransmit
	}
	return utils.WriteFrame(r.conn, delivered)
}

// writePayload appends payload bytes to the output file in order.
func (r *Receiver) writePayload(payload []byte) {
	r.outFile.Write(payload)
}



// ─── Protocols ────────────────────────────────────────────────────────────────
// The protocol implementations (RunStopWait, RunGBN, RunSR) are defined in:
//   - stop_wait.go: Stop-and-Wait ARQ
//   - gbn.go:       Go-Back-N ARQ (receiver window=1, strict sequential acceptance)
//   - sr.go:        Selective Repeat ARQ (receiver window=N, out-of-order buffering, optional NAK)


// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	var cfg Config
	if len(os.Args) > 1 {
		cfg = loadConfig(os.Args[1])
	} else {
		cfg = promptConfig()
	}

	// Open output file
	outFile, err := os.Create(cfg.OutputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create output file %q: %v\n", cfg.OutputFile, err)
		os.Exit(1)
	}
	defer outFile.Close()

	// Open CSV log
	logFile, csvW := openCSV(cfg.LogPrefix, cfg.Mode, cfg.LossProb)
	if logFile != nil {
		defer logFile.Close()
	}

	// Listen for incoming TCP connection
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot listen on %s: %v\n", cfg.ListenAddr, err)
		os.Exit(1)
	}
	fmt.Printf("[Receiver] Listening on %s (mode=%s, fcs=%s)\n",
		cfg.ListenAddr, cfg.Mode, cfg.FCSAlgo)

	conn, err := ln.Accept()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Accept failed: %v\n", err)
		os.Exit(1)
	}
	ln.Close() // Accept only one connection
	fmt.Printf("[Receiver] Connection from %s\n", conn.RemoteAddr())

	r := &Receiver{
		cfg:       cfg,
		conn:      conn,
		ch:        utils.NewChannel(cfg.LossProb, cfg.DelayProb, cfg.DelayDur, cfg.CorruptProb),
		outFile:   outFile,
		csvW:      csvW,
		startTime: time.Now(),
	}

	if err := r.Handshake(); err != nil {
		fmt.Fprintf(os.Stderr, "[Receiver] Channel handshake failed: %v\n", err)
		os.Exit(1)
	}

	switch cfg.Mode {
	case "gbn":
		r.RunGBN(cfg.WindowSize)
	case "sr":
		r.RunSR(cfg.WindowSize, cfg.NAKEnabled)
	default:
		r.RunStopWait()
	}

	r.printSummary()
}

// Handshake reads the channel configuration transmitted by the sender upon connection
// and applies those exact parameters to the receiver's local ACK channel (r.ch).
// This ensures ACK transmissions experience the exact same loss, delay, and corrupt
// impairments without requiring separate user inputs.
func (r *Receiver) Handshake() error {
	raw, err := utils.ReadDataFrame(r.conn)
	if err != nil {
		return fmt.Errorf("reading handshake frame: %w", err)
	}
	f, err := utils.Deserialize(raw, r.cfg.FCSAlgo)
	if err != nil {
		return fmt.Errorf("handshake FCS error: %w", err)
	}
	if f.Header.FrameType == utils.TypeConfig {
		loss, delay, dur, corrupt, err := utils.ParseConfigFrame(f)
		if err != nil {
			return fmt.Errorf("parsing channel config: %w", err)
		}
		r.cfg.LossProb = loss
		r.cfg.DelayProb = delay
		r.cfg.DelayDur = dur
		r.cfg.CorruptProb = corrupt
		r.ch = utils.NewChannel(loss, delay, dur, corrupt)
		fmt.Printf("[Receiver] Reused channel config from sender: loss=%.2f, delay=%.2f (%v), corrupt=%.2f\n",
			loss, delay, dur, corrupt)

		// Send positive ACK back to sender to confirm handshake
		ack := utils.NewACKFrame(ReceiverMAC, SenderMAC, 0)
		ackRaw, err := ack.Serialize(r.cfg.FCSAlgo)
		if err != nil {
			return err
		}
		return utils.WriteFrame(r.conn, ackRaw)
	}
	return fmt.Errorf("expected TypeConfig handshake, got frame type %d", f.Header.FrameType)
}
