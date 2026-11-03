package station

import (
	"ass3/common"
	"context"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// Station represents an independent network node contending for channel access.
type Station struct {
	ID          uint16
	ServerAddr  string
	Mode        string  // "non-persistent", "1-persistent", "p-persistent", "csma-cd"
	PVal        float64 // For p-persistent (default 0.5)
	TotalFrames int
	PayloadSize int
	Tp          time.Duration
	Tfr         time.Duration
	SlotTime    time.Duration
	IFG         time.Duration
	Quiet       bool

	conn    net.Conn
	rng     *rand.Rand
	rngMu   sync.Mutex
	metrics common.StationMetrics
}

// Config parameters for creating a new Station.
type Config struct {
	ID          uint16
	ServerAddr  string
	Mode        string
	PVal        float64
	TotalFrames int
	PayloadSize int
	Tp          time.Duration
	Tfr         time.Duration
	SlotTime    time.Duration
	IFG         time.Duration
	Quiet       bool
}

// NewStation creates an initialized Station instance.
func NewStation(cfg Config) *Station {
	if cfg.PayloadSize <= 0 {
		cfg.PayloadSize = common.DefaultPayloadSize
	}
	if cfg.Tp <= 0 {
		cfg.Tp = common.DefaultTp
	}
	if cfg.Tfr <= 0 {
		cfg.Tfr = common.DefaultTfr
	}
	if cfg.SlotTime <= 0 {
		cfg.SlotTime = 2 * cfg.Tp
	}
	if cfg.IFG <= 0 {
		cfg.IFG = common.DefaultIFG
	}
	if cfg.PVal <= 0 || cfg.PVal > 1.0 {
		cfg.PVal = 0.5
	}
	if cfg.TotalFrames <= 0 {
		cfg.TotalFrames = 50
	}

	seed := time.Now().UnixNano() + int64(cfg.ID)*1000003
	return &Station{
		ID:          cfg.ID,
		ServerAddr:  cfg.ServerAddr,
		Mode:        cfg.Mode,
		PVal:        cfg.PVal,
		TotalFrames: cfg.TotalFrames,
		PayloadSize: cfg.PayloadSize,
		Tp:          cfg.Tp,
		Tfr:         cfg.Tfr,
		SlotTime:    cfg.SlotTime,
		IFG:         cfg.IFG,
		Quiet:       cfg.Quiet,
		rng:         rand.New(rand.NewSource(seed)),
		metrics: common.StationMetrics{
			StationID: cfg.ID,
			Mode:      cfg.Mode,
		},
	}
}

// Connect establishes TCP connection with the Channel Server.
func (s *Station) Connect() error {
	conn, err := net.Dial("tcp", s.ServerAddr)
	if err != nil {
		return fmt.Errorf("station %d failed to connect to %s: %w", s.ID, s.ServerAddr, err)
	}
	s.conn = conn
	return nil
}

// Close gracefully closes the station's socket connection.
func (s *Station) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Metrics returns the station's recorded performance counters.
func (s *Station) Metrics() common.StationMetrics {
	return s.metrics
}

// SenseChannel sends OpSenseReq and blocks until OpRespIdle or OpRespBusy is received.
// Returns true if IDLE, false if BUSY.
func (s *Station) SenseChannel() (bool, error) {
	if s.conn == nil {
		return false, fmt.Errorf("station %d: not connected", s.ID)
	}

	req := common.SenseReq{StationID: s.ID}
	if err := common.WriteSenseReq(s.conn, req); err != nil {
		return false, fmt.Errorf("station %d: error writing sense request: %w", s.ID, err)
	}

	opcode, _, err := common.ReadMsg(s.conn)
	if err != nil {
		return false, fmt.Errorf("station %d: error reading sense response: %w", s.ID, err)
	}

	if opcode == common.OpRespIdle {
		return true, nil
	}
	return false, nil
}

// TransmitFrame executes the physical transmission cycle:
// 1. Sends CmdTxStart
// 2. Sleeps for Tfr (simulating transmission duration on wire)
// 3. Sends CmdTxEnd
// 4. Reads server verdict (OpRespSuccess or OpRespJam)
func (s *Station) TransmitFrame(seq uint32) (bool, error) {
	if s.conn == nil {
		return false, fmt.Errorf("station %d: not connected", s.ID)
	}

	startReq := common.TxStartReq{
		StationID:  s.ID,
		FrameSeq:   seq,
		PayloadLen: uint16(s.PayloadSize),
		TfrMs:      uint32(s.Tfr.Milliseconds()),
	}

	if err := common.WriteTxStartReq(s.conn, startReq); err != nil {
		return false, fmt.Errorf("station %d: error writing TxStart: %w", s.ID, err)
	}

	// Emulate wire transmission time
	time.Sleep(s.Tfr)

	endReq := common.TxEndReq{
		StationID: s.ID,
		FrameSeq:  seq,
	}

	if err := common.WriteTxEndReq(s.conn, endReq); err != nil {
		return false, fmt.Errorf("station %d: error writing TxEnd: %w", s.ID, err)
	}

	opcode, payload, err := common.ReadMsg(s.conn)
	if err != nil {
		return false, fmt.Errorf("station %d: error reading TxResult: %w", s.ID, err)
	}

	resp, err := common.ReadTxResultResp(opcode, payload)
	if err != nil {
		return false, fmt.Errorf("station %d: failed to parse TxResult: %w", s.ID, err)
	}

	isSuccess := (resp.Opcode == common.OpRespSuccess)
	return isSuccess, nil
}

func (s *Station) randInt(n int) int {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Intn(n)
}

func (s *Station) randFloat() float64 {
	s.rngMu.Lock()
	defer s.rngMu.Unlock()
	return s.rng.Float64()
}

// Run executes the MAC algorithm configured for this station.
func (s *Station) Run(ctx context.Context) error {
	switch s.Mode {
	case "non-persistent":
		return s.runNonPersistent(ctx)
	case "1-persistent":
		return s.run1Persistent(ctx)
	case "p-persistent":
		return s.runPPersistent(ctx)
	case "csma-cd":
		return s.runCSMACD(ctx)
	default:
		return fmt.Errorf("unsupported MAC mode: %s", s.Mode)
	}
}
