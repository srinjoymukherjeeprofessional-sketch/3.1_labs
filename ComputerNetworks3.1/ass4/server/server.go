package server

import (
	"ass4/common"
	"fmt"
	"net"
	"sync"
)

// StationClient represents an active station connected to the channel.
type StationClient struct {
	ID        uint16
	Conn      net.Conn
	WalshCode []int8
}

// ChannelServer orchestrates CDMA multi-access over a common channel.
type ChannelServer struct {
	listenAddr  string
	numStations int
	codeLen     int // M = NextPowerOf2(numStations)

	walshMatrix [][]int8
	listener    net.Listener
	mu          sync.Mutex

	stations map[uint16]*StationClient
	quit     chan struct{}
}

// NewChannelServer creates a new ChannelServer instance.
func NewChannelServer(addr string, numStations int) (*ChannelServer, error) {
	if numStations <= 0 {
		numStations = 2
	}

	m := common.NextPowerOf2(numStations)
	matrix, err := common.GenerateWalshMatrix(m)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Walsh matrix: %w", err)
	}

	return &ChannelServer{
		listenAddr:  addr,
		numStations: numStations,
		codeLen:     m,
		walshMatrix: matrix,
		stations:    make(map[uint16]*StationClient),
		quit:        make(chan struct{}),
	}, nil
}

// Addr returns the server's listening network address.
func (s *ChannelServer) Addr() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.listenAddr
}

// Start begins listening and blocks while running the CDMA round barrier loop.
func (s *ChannelServer) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("channel server listen error: %w", err)
	}
	s.listener = ln

	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║               CDMA COMMON CHANNEL SERVER (WALSH CODES)            ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  Listening Address    : %s\n", ln.Addr())
	fmt.Printf("  Expected Stations (N): %d\n", s.numStations)
	fmt.Printf("  Walsh Code Size   (M): %d\n", s.codeLen)
	fmt.Println("  Generated Walsh Matrix H_M:")
	for i, row := range s.walshMatrix {
		if i < s.numStations {
			fmt.Printf("    Station %d (W_%d): %v\n", i+1, i+1, row)
		} else {
			fmt.Printf("    Unused    (W_%d): %v\n", i+1, row)
		}
	}
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n Waiting for %d stations to connect and register...\n", s.numStations)

	// Phase 1: Wait for all N stations to connect and register
	assignedID := uint16(1)
	for int(assignedID) <= s.numStations {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-s.quit:
				return nil
			default:
				return fmt.Errorf("accept error: %w", err)
			}
		}

		// Read registration message
		op, _, err := common.ReadMsg(conn)
		if err != nil || op != common.OpRegisterReq {
			_ = conn.Close()
			continue
		}

		stnID := assignedID
		assignedID++

		walshCode := s.walshMatrix[stnID-1]
		client := &StationClient{
			ID:        stnID,
			Conn:      conn,
			WalshCode: walshCode,
		}

		s.mu.Lock()
		s.stations[stnID] = client
		s.mu.Unlock()

		// Send registration response with assigned ID and Walsh code
		resp := common.RegisterResp{
			AssignedID:    stnID,
			CodeLen:       uint16(s.codeLen),
			TotalStations: uint16(s.numStations),
			WalshCode:     walshCode,
		}
		if err := common.WriteRegisterResp(conn, resp); err != nil {
			return fmt.Errorf("failed to send register response: %w", err)
		}

		fmt.Printf("  [+] Station %d connected from %s | Assigned Walsh Code W_%d: %v\n",
			stnID, conn.RemoteAddr(), stnID, walshCode)
	}

	fmt.Println("\n===================================================================")
	fmt.Printf("All %d stations connected and ready. Entering CDMA slotted rounds!\n", s.numStations)
	fmt.Println("===================================================================")

	// Phase 2: Run synchronized round barrier loop
	return s.runRoundLoop()
}

// runRoundLoop executes the synchronous slotted rounds.
// In each round:
// 1. Server notifies all stations that a round has started.
// 2. Server waits for all stations to submit their chip sequence.
// 3. Server computes linear superposition C = SUM(chips).
// 4. Server broadcasts C to all stations.
func (s *ChannelServer) runRoundLoop() error {
	roundSeq := uint32(1)

	for {
		select {
		case <-s.quit:
			return nil
		default:
		}

		fmt.Printf("\n─── [Round %d] ───────────────────────────────────────────────────\n", roundSeq)
		fmt.Printf("Signaling %d stations to transmit their bits/chips...\n", s.numStations)

		// 1. Notify stations of round start
		s.mu.Lock()
		for _, client := range s.stations {
			if err := common.WriteRoundStart(client.Conn, common.RoundStart{RoundSeq: roundSeq}); err != nil {
				s.mu.Unlock()
				return fmt.Errorf("station %d round start error: %w", client.ID, err)
			}
		}
		s.mu.Unlock()

		// 2. Barrier: Collect chip sequence from each station
		type chipResult struct {
			stationID uint16
			chips     []int8
			err       error
		}
		resultsCh := make(chan chipResult, s.numStations)

		for _, client := range s.stations {
			go func(c *StationClient) {
				op, payload, err := common.ReadMsg(c.Conn)
				if err != nil {
					resultsCh <- chipResult{stationID: c.ID, err: err}
					return
				}
				if op == common.OpQuit {
					resultsCh <- chipResult{stationID: c.ID, err: fmt.Errorf("station %d disconnected", c.ID)}
					return
				}
				if op != common.OpChipTx {
					resultsCh <- chipResult{stationID: c.ID, err: fmt.Errorf("unexpected opcode 0x%02x", op)}
					return
				}
				tx, err := common.ReadChipTx(payload)
				if err != nil {
					resultsCh <- chipResult{stationID: c.ID, err: err}
					return
				}
				resultsCh <- chipResult{stationID: c.ID, chips: tx.Chips}
			}(client)
		}

		stationChips := make(map[uint16][]int8)
		var collectedVectors [][]int8

		for i := 0; i < s.numStations; i++ {
			res := <-resultsCh
			if res.err != nil {
				return fmt.Errorf("error waiting for chips: %w", res.err)
			}
			stationChips[res.stationID] = res.chips
			fmt.Printf("  <- Received chips from Station %d: %v\n", res.stationID, res.chips)
		}

		for id := uint16(1); id <= uint16(s.numStations); id++ {
			collectedVectors = append(collectedVectors, stationChips[id])
		}

		// 3. Channel Linear Superposition (Addition)
		composite := common.Superposition(collectedVectors, s.codeLen)

		fmt.Println("  -------------------------------------------------------------")
		fmt.Printf("  [Superposition] Composite Channel Vector S: %v\n", composite)
		fmt.Println("  -------------------------------------------------------------")

		// 4. Broadcast composite vector to all stations
		bcast := common.ChannelBroadcast{
			RoundSeq:  roundSeq,
			Composite: composite,
		}

		s.mu.Lock()
		for _, client := range s.stations {
			if err := common.WriteChannelBroadcast(client.Conn, bcast); err != nil {
				s.mu.Unlock()
				return fmt.Errorf("failed to broadcast to station %d: %w", client.ID, err)
			}
		}
		s.mu.Unlock()

		fmt.Printf("  => Composite vector broadcasted to all %d stations.\n", s.numStations)
		roundSeq++
	}
}

// Close terminates the server and all client connections.
func (s *ChannelServer) Close() {
	close(s.quit)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, client := range s.stations {
		_ = client.Conn.Close()
	}
}
