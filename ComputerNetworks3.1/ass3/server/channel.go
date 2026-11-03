package server

import (
	"ass3/common"
	"sync"
	"time"
)

// ChannelState represents the physical state of the shared medium.
type ChannelState int

const (
	StateIdle ChannelState = iota
	StateBusy
	StateCollision
)

func (s ChannelState) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateBusy:
		return "BUSY"
	case StateCollision:
		return "COLLISION"
	default:
		return "UNKNOWN"
	}
}

// ActiveTx records an in-flight frame transmission on the wire.
type ActiveTx struct {
	StationID  uint16
	FrameSeq   uint32
	PayloadLen uint16
	Tfr        time.Duration
	StartTime  time.Time
	Collided   bool
}

// Channel manages the state machine and physical constraints (Tp, Tfr).
type Channel struct {
	mu sync.Mutex

	tp time.Duration // One-way propagation delay (vulnerability window)

	state          ChannelState
	firstTxStart   time.Time
	activeTx       map[uint16]*ActiveTx
	collisionCount int

	// Metrics
	totalAttempts    int
	totalSuccess     int
	totalCollisions  int
	totalPayloadBits int64
	startTime        time.Time
	quiet            bool
}

// NewChannel creates an initialized Channel.
func NewChannel(tp time.Duration, quiet bool) *Channel {
	if tp <= 0 {
		tp = common.DefaultTp
	}
	return &Channel{
		tp:        tp,
		state:     StateIdle,
		activeTx:  make(map[uint16]*ActiveTx),
		startTime: time.Now(),
		quiet:     quiet,
	}
}

// Reset clears state and counters for a new benchmark run.
func (c *Channel) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.state = StateIdle
	c.activeTx = make(map[uint16]*ActiveTx)
	c.collisionCount = 0
	c.totalAttempts = 0
	c.totalSuccess = 0
	c.totalCollisions = 0
	c.totalPayloadBits = 0
	c.startTime = time.Now()
}

// HandleSense evaluates what the channel looks like to a station probing it.
// Returns true if IDLE, false if BUSY.
// Physics: If Station A started transmitting at t0, any probe arriving
// during [t0, t0 + Tp) still sees IDLE because the signal has not reached it.
func (c *Channel) HandleSense(stationID uint16) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state == StateIdle {
		return true
	}

	// Channel is BUSY or COLLISION.
	// Check if we are still within the vulnerability window of the first transmitter.
	if c.state == StateBusy {
		elapsed := time.Since(c.firstTxStart)
		if elapsed < c.tp {
			// Signal has not propagated to all stations yet!
			return true
		}
	}

	return false
}

// HandleTxStart processes a station beginning frame transmission.
func (c *Channel) HandleTxStart(req common.TxStartReq) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalAttempts++
	now := time.Now()
	tx := &ActiveTx{
		StationID:  req.StationID,
		FrameSeq:   req.FrameSeq,
		PayloadLen: req.PayloadLen,
		Tfr:        time.Duration(req.TfrMs) * time.Millisecond,
		StartTime:  now,
		Collided:   false,
	}

	if c.state == StateIdle {
		// First transmitter on an idle medium
		c.state = StateBusy
		c.firstTxStart = now
		c.activeTx[req.StationID] = tx
		return
	}

	// If another transmission is already active, this is a COLLISION!
	// This happens either because the station sensed during [t0, t0 + Tp)
	// or failed to sense / transmitted regardless.
	c.state = StateCollision
	c.totalCollisions++
	tx.Collided = true

	// Mark all other active transmitters as collided as well
	for _, other := range c.activeTx {
		other.Collided = true
	}

	c.activeTx[req.StationID] = tx
}

// HandleTxEnd processes a station finishing frame transmission.
// Returns opcode (OpRespSuccess or OpRespJam) and current collision count.
func (c *Channel) HandleTxEnd(req common.TxEndReq) (byte, uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	tx, exists := c.activeTx[req.StationID]
	if !exists {
		// Unknown transmission, treat as error/jam
		return common.OpRespJam, uint32(c.totalCollisions)
	}

	delete(c.activeTx, req.StationID)

	collided := tx.Collided
	if !collided {
		c.totalSuccess++
		c.totalPayloadBits += int64(tx.PayloadLen) * 8
	}

	// If no more active transmitters on the wire, return channel to IDLE
	if len(c.activeTx) == 0 {
		c.state = StateIdle
	}

	if collided {
		return common.OpRespJam, uint32(c.totalCollisions)
	}
	return common.OpRespSuccess, uint32(c.totalCollisions)
}

// HandleDisconnect cleans up state if a station abruptly closes TCP connection.
func (c *Channel) HandleDisconnect(stationID uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.activeTx, stationID)
	if len(c.activeTx) == 0 {
		c.state = StateIdle
	}
}

// SnapshotMetrics collects the current simulation metrics.
func (c *Channel) SnapshotMetrics(mode string, numStations int, tfr time.Duration, pVal float64) common.SimulationMetrics {
	c.mu.Lock()
	defer c.mu.Unlock()

	dur := time.Since(c.startTime)
	m := common.SimulationMetrics{
		Mode:             mode,
		PValue:           pVal,
		NumStations:      numStations,
		Tp:               c.tp,
		Tfr:              tfr,
		SlotTime:         2 * c.tp,
		Duration:         dur,
		TotalAttempts:    c.totalAttempts,
		TotalSuccess:     c.totalSuccess,
		TotalCollisions:  c.totalCollisions,
		TotalDropped:     0, // Dropped frames tracked by stations and combined in orchestrator
		TotalPayloadBits: c.totalPayloadBits,
	}
	m.ComputeDerivedMetrics()
	return m
}
