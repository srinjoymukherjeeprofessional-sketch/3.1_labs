// sender/main.go
//
// Sender program for Assignment 2 — Data Link Layer Simulation.
//
// ─── Overview ────────────────────────────────────────────────────────────────
// The Sender reads an input file, splits it into PayloadSize-byte chunks, wraps
// each chunk in a Data Link Layer frame, applies a simulated unreliable channel,
// and transmits the frame to the Receiver over a TCP connection.
//
// Three flow-control protocols are supported:
//   stop-wait  Stop-and-Wait ARQ
//   gbn        Go-Back-N ARQ  (window size N)
//   sr         Selective Repeat ARQ  (window size N, optional NAK)
//
// ─── Adaptive Timeout 
// 1. Initial RTO : 1 second
// 2. Average of previous RTT and SRTT
// 3. RTO = 2 * SRTT

// ─── Channel Simulation ──────────────────────────────────────────────────────
// Before writing a serialized frame to the TCP socket, utils.Channel.Transmit()
// is called.  It can:
//   • Drop the frame  (returns dropped=true  → skip the Write call)
//   • Delay the frame (calls time.Sleep)
//   • Corrupt the frame (flips a random bit via InjectSingleBitError)
// From the Receiver's perspective this looks exactly like a real lossy link.

// ─── Configuration ───────────────────────────────────────────────────────────
// If a config-file path is supplied as os.Args[1], settings are read from it
// (key=value, one per line — useful for scripted eval runs).
// Otherwise the program prompts the user interactively.
package main

import (
	"ass2/utils"
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── Constants ────────────────────────────────────────────────────────────────

const (
	PayloadSize = 64 // Fixed payload size in bytes (46–1500 range per spec)
	SeqModulus  = 64 // SeqNum wraps at 64 (6-bit field in control byte)
)

// Hardcoded MAC addresses — both sides use the same pair (sender ↔ receiver).
var (
	SenderMAC   = [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x01}
	ReceiverMAC = [6]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0x02}
)

// ─── Configuration ────────────────────────────────────────────────────────────

// Config holds all runtime parameters for the Sender.
type Config struct {
	RemoteAddr  string        // Receiver TCP address e.g. "localhost:9000"
	Mode        string        // "stop-wait" | "gbn" | "sr"
	FCSAlgo     string        // "crc32" | "checksum"
	WindowSize  int           // N for GBN / SR (ignored for stop-wait)
	NAKEnabled  bool          // SR only: send immediate NAK on out-of-order
	LossProb    float64       // Channel packet-loss probability [0, 1]
	DelayProb   float64       // Channel artificial-delay probability [0, 1]
	DelayDur    time.Duration // Duration of artificial delay when triggered
	CorruptProb float64       // Channel bit-corruption probability [0, 1]
	InputFile   string        // Path to file being transmitted
	LogPrefix   string        // Prefix for output CSV logs (default "sender")
}

// promptConfig interactively collects parameters from the user.
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
	parseFloat := func(s string) float64 { v, _ := strconv.ParseFloat(s, 64); return v }
	parseInt := func(s string) int { v, _ := strconv.Atoi(s); return v }

	cfg := Config{}
	cfg.RemoteAddr = ask("Receiver address (host:port)", "localhost:9000")
	cfg.Mode = ask("Mode (stop-wait / gbn / sr)", "stop-wait")
	cfg.FCSAlgo = ask("FCS algorithm (crc32 / checksum)", "crc32")
	cfg.InputFile = ask("Input file path", "input.txt")
	cfg.LogPrefix = ask("Log CSV prefix", "sender")

	if cfg.Mode != "stop-wait" {
		cfg.WindowSize = parseInt(ask("Window size N", "4"))
	}
	if cfg.Mode == "sr" {
		cfg.NAKEnabled = ask("Enable NAK? (y/n)", "n") == "y"
	}

	cfg.LossProb = parseFloat(ask("Channel loss probability (0.0–1.0)", "0.0"))
	cfg.DelayProb = parseFloat(ask("Channel delay probability (0.0–1.0)", "0.0"))
	durMs := parseInt(ask("Channel delay duration ms", "100"))
	cfg.DelayDur = time.Duration(durMs) * time.Millisecond
	cfg.CorruptProb = parseFloat(ask("Channel corrupt probability (0.0–1.0)", "0.0"))
	return cfg
}

// loadConfig reads a config file (key=value per line).
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
	pf := func(k string, def float64) float64 {
		if v, err := strconv.ParseFloat(m[k], 64); err == nil && m[k] != "" {
			return v
		}
		return def
	}
	pi := func(k string, def int) int { v, _ := strconv.Atoi(m[k]); if m[k] == "" { return def }; return v }
	pb := func(k string) bool { return m[k] == "true" || m[k] == "y" }
	str := func(k, def string) string { if v := m[k]; v != "" { return v }; return def }

	durMs := pi("delay_dur_ms", 100)
	return Config{
		RemoteAddr:  str("remote", "localhost:9000"),
		Mode:        str("mode", "stop-wait"),
		FCSAlgo:     str("fcs", "crc32"),
		WindowSize:  pi("window", 4),
		NAKEnabled:  pb("nak"),
		LossProb:    pf("loss", 0),
		DelayProb:   pf("delay", 0),
		DelayDur:    time.Duration(durMs) * time.Millisecond,
		CorruptProb: pf("corrupt", 0),
		InputFile:   str("input", "input.txt"),
		LogPrefix:   str("log_prefix", "sender"),
	}
}

// ─── Sender struct ────────────────────────────────────────────────────────────

// Sender encapsulates all state for the sending side.
type Sender struct {
	cfg      Config
	conn     net.Conn       // TCP connection to the receiver
	ch       *utils.Channel // Simulated unreliable channel
	rto      *RTOState      // Adaptive timeout state
	payloads [][]byte       // File contents split into PayloadSize-byte chunks
	csvW     *csv.Writer    // Per-frame log writer
	// Cumulative statistics
	mu       sync.Mutex
	framesSent  int
	retransmits int
	startTime   time.Time
}

// ─── Sender methods ───────────────────────────────────────────────────────────

// Framing wraps a raw payload into a properly constructed Frame.
func (s *Sender) Framing(payload []byte, seqNum uint8, frameType byte) *utils.Frame {
	switch frameType {
	case utils.TypeACK:
		return utils.NewACKFrame(SenderMAC, ReceiverMAC, seqNum)
	default:
		return utils.NewDataFrame(SenderMAC, ReceiverMAC, seqNum, payload)
	}
}

// Send serializes a frame, passes it through the channel simulation, and
// writes it to the TCP connection (unless the channel drops it).
// Returns (dropped, error) — if dropped==true the frame was silently lost.
func (s *Sender) Send(frame *utils.Frame) (bool, error) {
	raw, err := frame.Serialize(s.cfg.FCSAlgo)
	if err != nil {
		return false, err
	}
	delivered, dropped, _ := s.ch.Transmit(raw)
	if dropped {
		s.mu.Lock()
		s.framesSent++
		s.mu.Unlock()
		return true, nil
	}
	if err := utils.WriteFrame(s.conn, delivered); err != nil {
		return false, err
	}
	s.mu.Lock()
	s.framesSent++
	s.mu.Unlock()
	return false, nil
}

// Recv reads one frame from the TCP connection (used to receive ACK/NAK).
// Returns nil if the frame is corrupted (FCS mismatch) — caller should treat
// this the same as a timeout (no valid ACK received).
func (s *Sender) Recv() (*utils.Frame, error) {
	raw, err := utils.ReadACKFrame(s.conn)
	if err != nil {
		return nil, err
	}
	frame, err := utils.Deserialize(raw, s.cfg.FCSAlgo)
	if err != nil {
		// FCS mismatch — corrupted ACK, caller handles as missing ACK
		return nil, nil
	}
	return frame, nil
}


// ─── Protocols ────────────────────────────────────────────────────────────────
// The protocol implementations (RunStopWait, RunGBN, RunSR) are defined in:
//   - stop_wait.go: Stop-and-Wait ARQ
//   - gbn.go:       Go-Back-N ARQ (sliding window size N, cumulative ACK)
//   - sr.go:        Selective Repeat ARQ (window size N, individual ACK, optional NAK)

// ─── Helpers ──────────────────────────────────────────────────────────────────


// readPayloads opens the input file and slices it into PayloadSize-byte chunks.
func readPayloads(path string) ([][]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var chunks [][]byte
	for len(data) > 0 {
		size := PayloadSize
		if len(data) < size {
			size = len(data)
		}
		chunks = append(chunks, data[:size])
		data = data[size:]
	}
	return chunks, nil
}


// ─── main ─────────────────────────────────────────────────────────────────────

func main() {
	// Load config: from file if provided, otherwise interactive prompts
	var cfg Config
	if len(os.Args) > 1 {
		cfg = loadConfig(os.Args[1])
	} else {
		cfg = promptConfig()
	}

	// Read input file into payload chunks
	payloads, err := readPayloads(cfg.InputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read input file %q: %v\n", cfg.InputFile, err)
		os.Exit(1)
	}
	fmt.Printf("[Sender] %d payload chunks of %d bytes each\n", len(payloads), PayloadSize)

	// Open CSV log
	logFile, csvW := openCSV(cfg.LogPrefix, cfg.Mode, cfg.LossProb)
	if logFile != nil {
		defer logFile.Close()
	}

	// Connect to receiver
	conn, err := net.Dial("tcp", cfg.RemoteAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot connect to %s: %v\n", cfg.RemoteAddr, err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("[Sender] Connected to %s\n", cfg.RemoteAddr)

	// Build Sender
	s := &Sender{
		cfg:      cfg,
		conn:     conn,
		ch:       utils.NewChannel(cfg.LossProb, cfg.DelayProb, cfg.DelayDur, cfg.CorruptProb),
		rto:      newRTOState(),
		payloads: payloads,
		csvW:     csvW,
		startTime: time.Now(),
	}

	// Perform channel configuration handshake with receiver
	if err := s.Handshake(); err != nil {
		fmt.Fprintf(os.Stderr, "[Sender] Channel config handshake failed: %v\n", err)
		os.Exit(1)
	}

	// Run the selected protocol
	t0 := time.Now()
	switch cfg.Mode {
	case "gbn":
		s.RunGBN(cfg.WindowSize)
	case "sr":
		s.RunSR(cfg.WindowSize, cfg.NAKEnabled)
	default:
		s.RunStopWait()
	}
	elapsed := time.Since(t0)

	s.printSummary(elapsed)
}

// Handshake transmits the sender's channel configuration to the receiver upon connection.
// This allows the receiver to mirror the exact same channel settings on the ACK path
// without requiring separate user inputs.
func (s *Sender) Handshake() error {
	frame := utils.NewConfigFrame(SenderMAC, ReceiverMAC, s.cfg.LossProb, s.cfg.DelayProb, s.cfg.DelayDur, s.cfg.CorruptProb)
	raw, err := frame.Serialize(s.cfg.FCSAlgo)
	if err != nil {
		return err
	}
	if err := utils.WriteFrame(s.conn, raw); err != nil {
		return err
	}
	ackRaw, err := utils.ReadACKFrame(s.conn)
	if err != nil {
		return fmt.Errorf("waiting for handshake ACK: %w", err)
	}
	ack, err := utils.Deserialize(ackRaw, s.cfg.FCSAlgo)
	if err != nil || ack == nil || ack.Header.FrameType != utils.TypeACK {
		return errors.New("channel config handshake was not acknowledged")
	}
	fmt.Printf("[Sender] Channel config synced with receiver (loss=%.2f, delay=%.2f, corrupt=%.2f)\n",
		s.cfg.LossProb, s.cfg.DelayProb, s.cfg.CorruptProb)
	return nil
}
