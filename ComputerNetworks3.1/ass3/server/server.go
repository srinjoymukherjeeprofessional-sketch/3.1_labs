package server

import (
	"ass3/common"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// Server coordinates the TCP interface to the simulated physical channel.
type Server struct {
	addr     string
	listener net.Listener
	channel  *Channel

	activeConnCount int32
	closed          int32
	wg              sync.WaitGroup
	quiet           bool
}

// NewServer creates a Server bound to addr with propagation delay tp.
func NewServer(addr string, tp time.Duration, quiet bool) *Server {
	return &Server{
		addr:    addr,
		channel: NewChannel(tp, quiet),
		quiet:   quiet,
	}
}

// Channel returns the underlying Channel instance.
func (s *Server) Channel() *Channel {
	return s.channel
}

// Start begins listening on TCP and serving station connections.
func (s *Server) Start() error {
	l, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to bind server on %s: %w", s.addr, err)
	}
	s.listener = l
	s.addr = l.Addr().String()

	s.wg.Add(1)
	go s.acceptLoop()
	return nil
}

// Addr returns the bound TCP address (useful when port :0 is passed for dynamic port allocation).
func (s *Server) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.addr
}

// Close gracefully terminates listener and waits for active connections to finish.
func (s *Server) Close() error {
	if !atomic.CompareAndSwapInt32(&s.closed, 0, 1) {
		return nil
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if atomic.LoadInt32(&s.closed) != 0 {
				return
			}
			continue
		}

		atomic.AddInt32(&s.activeConnCount, 1)
		s.wg.Add(1)
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		atomic.AddInt32(&s.activeConnCount, -1)
		s.wg.Done()
	}()

	var stationID uint16

	for {
		opcode, payload, err := common.ReadMsg(conn)
		if err != nil {
			if err != io.EOF && atomic.LoadInt32(&s.closed) == 0 && !s.quiet {
				// connection dropped
			}
			if stationID != 0 {
				s.channel.HandleDisconnect(stationID)
			}
			return
		}

		switch opcode {
		case common.OpSenseReq:
			req, err := common.ReadSenseReq(payload)
			if err != nil {
				continue
			}
			stationID = req.StationID
			isIdle := s.channel.HandleSense(req.StationID)
			if err := common.WriteSenseResp(conn, isIdle); err != nil {
				return
			}

		case common.OpTxStartReq:
			req, err := common.ReadTxStartReq(payload)
			if err != nil {
				continue
			}
			stationID = req.StationID
			s.channel.HandleTxStart(req)

		case common.OpTxEndReq:
			req, err := common.ReadTxEndReq(payload)
			if err != nil {
				continue
			}
			stationID = req.StationID
			verdictOp, colCount := s.channel.HandleTxEnd(req)
			resp := common.TxResultResp{
				Opcode:         verdictOp,
				FrameSeq:       req.FrameSeq,
				CollisionCount: colCount,
			}
			if err := common.WriteTxResultResp(conn, resp); err != nil {
				return
			}
		}
	}
}
