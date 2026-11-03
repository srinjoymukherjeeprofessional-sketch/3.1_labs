package station

/*
================================================================================
CDMA Station Implementation Guide & Skeleton
================================================================================

This file contains the commented skeleton for the CDMA Station client.
You can fill in or uncomment the implementation as you build your station logic.

Key Responsibilities of a Station:
1. Connect over TCP to the Channel Server.
2. Send OpRegisterReq and receive OpRegisterResp containing:
   - Station ID (e.g., 1, 2, ... N)
   - Code length M
   - Unique assigned Walsh code W_i (length M, values +1 or -1)
   - Total number of stations N
3. Interactive Slotted Round Loop:
   a. Wait for OpRoundStart from server (indicates a new round has started).
   b. Prompt user for bit to transmit:
      - '1' -> Bipolar +1
      - '0' -> Bipolar -1
      - 's' or Enter -> Bipolar 0 (Silent / No transmission)
   c. Chip Modulation:
      - Multiply data bipolar value d_i with Walsh code W_i:
        Chips = [d_i * W_i[0], d_i * W_i[1], ..., d_i * W_i[M-1]]
   d. Transmit OpChipTx to server.
   e. Wait for OpChannelBroadcast from server containing composite vector S:
      - S is the linear sum of all active stations' chips: S = SUM(d_k * W_k)
   f. Reception & Reconstruct Data:
      - Reconstruct data sent by every station k in 1..N:
        InnerProduct = (S . W_k)
        Decision = InnerProduct / M
        If Decision == +1 -> Station k sent bit 1
        If Decision == -1 -> Station k sent bit 0
        If Decision == 0  -> Station k was silent
   g. Display results in terminal and loop to next round.
================================================================================
*/

import (
	"ass4/common"
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

// Station represents a CDMA terminal client.
type Station struct {
	serverAddr    string
	id            uint16
	codeLen       uint16
	totalStations uint16
	walshCode     []int8
	conn          net.Conn
}

// NewStation creates an initialized Station pointer.
func NewStation(serverAddr string) *Station {
	return &Station{
		serverAddr: serverAddr,
	}
}

// Connect dials the Channel Server and performs CDMA registration.
func (s *Station) Connect() error {
	conn, err := net.Dial("tcp", s.serverAddr)
	if err != nil {
		return fmt.Errorf("failed to dial server %s: %w", s.serverAddr, err)
	}
	s.conn = conn

	// 1. Send OpRegisterReq
	req := common.RegisterReq{RequestedID: 0}
	if err := common.WriteRegisterReq(s.conn, req); err != nil {
		return fmt.Errorf("failed to send register request: %w", err)
	}

	// 2. Read OpRegisterResp
	op, payload, err := common.ReadMsg(s.conn)
	if err != nil {
		return fmt.Errorf("failed to read register response: %w", err)
	}
	if op != common.OpRegisterResp {
		return fmt.Errorf("expected OpRegisterResp, got 0x%02x", op)
	}

	resp, err := common.ReadRegisterResp(payload)
	if err != nil {
		return fmt.Errorf("failed to parse register response: %w", err)
	}

	s.id = resp.AssignedID
	s.codeLen = resp.CodeLen
	s.totalStations = resp.TotalStations
	s.walshCode = resp.WalshCode

	fmt.Println("╔═══════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║                     CONNECTED AS STATION %d                        ║\n", s.id)
	fmt.Println("╠═══════════════════════════════════════════════════════════════════╣")
	fmt.Printf("  Assigned Station ID : %d\n", s.id)
	fmt.Printf("  Total Stations (N)  : %d\n", s.totalStations)
	fmt.Printf("  Walsh Code Size (M) : %d\n", s.codeLen)
	fmt.Printf("  Assigned Walsh Code : %v\n", s.walshCode)
	fmt.Println("╚═══════════════════════════════════════════════════════════════════╝")
	return nil
}

// RunInteractiveLoop handles the round-by-round interactive bit transmission and reception.
func (s *Station) RunInteractiveLoop() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		// Step A: Wait for OpRoundStart from server
		op, payload, err := common.ReadMsg(s.conn)
		if err != nil {
			return fmt.Errorf("error reading from server: %w", err)
		}
		if op == common.OpQuit {
			fmt.Println("Server closed connection.")
			return nil
		}
		if op != common.OpRoundStart {
			return fmt.Errorf("expected OpRoundStart, got 0x%02x", op)
		}

		roundInfo, err := common.ReadRoundStart(payload)
		if err != nil {
			return err
		}

		fmt.Printf("\n─── [Round %d] ───────────────────────────────────────────────────\n", roundInfo.RoundSeq)
		fmt.Printf("Enter bit to transmit for Station %d (1 = bit 1, 0 = bit 0, s = silent): ", s.id)

		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(line)

		var bipolar int8
		switch line {
		case "1":
			bipolar = 1
		case "0":
			bipolar = -1
		default:
			bipolar = 0 // silent
		}

		// Step B: Encode bit into chip sequence
		chips := common.EncodeBit(bipolar, s.walshCode)
		if bipolar == 0 {
			fmt.Println("  -> [Tx] Station is SILENT. Sending all-zero chip vector.")
		} else {
			bitVal := 0
			if bipolar == 1 {
				bitVal = 1
			}
			fmt.Printf("  -> [Tx] Sending Bit %d (bipolar %+d) * W_%d = %v\n", bitVal, bipolar, s.id, chips)
		}

		// Step C: Send chips to server
		txMsg := common.ChipTx{
			StationID: s.id,
			RoundSeq:  roundInfo.RoundSeq,
			Chips:     chips,
		}
		if err := common.WriteChipTx(s.conn, txMsg); err != nil {
			return fmt.Errorf("failed to send chips: %w", err)
		}

		fmt.Println("  Waiting for other stations to transmit...")

		// Step D: Read OpChannelBroadcast containing composite channel vector
		op, payload, err = common.ReadMsg(s.conn)
		if err != nil {
			return fmt.Errorf("failed to read channel broadcast: %w", err)
		}
		if op != common.OpChannelBroadcast {
			return fmt.Errorf("expected OpChannelBroadcast, got 0x%02x", op)
		}

		bcast, err := common.ReadChannelBroadcast(payload)
		if err != nil {
			return err
		}

		// Step E: Demultiplex and reconstruct data
		fmt.Println("  -------------------------------------------------------------")
		fmt.Printf("  [Rx] Channel Composite Signal S: %v\n", bcast.Composite)

		// Decode our own bit:
		myDecoded, mySum := common.DecodeChip(bcast.Composite, s.walshCode)
		fmt.Printf("  [Self-Check] Normalized Inner Product (S . W_%d) / %d = %+d (Sum: %d)\n",
			s.id, s.codeLen, myDecoded, mySum)

		// switch myDecoded {
		// case 1:
		// 	fmt.Printf("  => Reconstructed for Station %d: Bit 1\n", s.id)
		// case -1:
		// 	fmt.Printf("  => Reconstructed for Station %d: Bit 0\n", s.id)
		// case 0:
		// 	fmt.Printf("  => Reconstructed for Station %d: Silent (No data)\n", s.id)
		// }

		// If you also want to reconstruct what all other stations sent:
		// You can regenerate the Walsh Matrix using common.GenerateWalshMatrix(s.codeLen)
		// and decode against W_k for each station k in 1..N!
		walshMatrix, _ := common.GenerateWalshMatrix(int(s.codeLen))
		fmt.Println("  [All Stations Decoded]:")
		for stn := 1; stn <= int(s.totalStations); stn++ {
			dec, _ := common.DecodeChip(bcast.Composite, walshMatrix[stn-1])
			status := "Silent"
			if dec == 1 {
				status = "Bit 1"
			} else if dec == -1 {
				status = "Bit 0"
			}
			fmt.Printf("     Station %d: %s\n", stn, status)
		}
		fmt.Println("  -------------------------------------------------------------")
	}
}

// Close disconnects the station.
func (s *Station) Close() {
	if s.conn != nil {
		_ = common.WriteMsg(s.conn, common.OpQuit, nil)
		_ = s.conn.Close()
	}
}

