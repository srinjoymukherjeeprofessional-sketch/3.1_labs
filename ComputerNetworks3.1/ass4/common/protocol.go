package common

import (
	"encoding/binary"
	"fmt"
	"io"
)

// ─── Protocol Opcodes ──────────────────────────────────────────────────────────

const (
	OpRegisterReq        byte = 0x01 // Station -> Server: Request to join CDMA channel
	OpRegisterResp       byte = 0x02 // Server -> Station: Assigned ID, M, Walsh code
	OpRoundStart         byte = 0x03 // Server -> Station: Trigger next input round
	OpChipTx             byte = 0x04 // Station -> Server: Transmit chip sequence for current round
	OpChannelBroadcast   byte = 0x05 // Server -> Station: Superimposed composite channel vector
	OpQuit               byte = 0x06 // Station <-> Server: Clean disconnect
)

// ─── Protocol Structs ─────────────────────────────────────────────────────────

// RegisterReq is sent by a station upon initial TCP connection.
type RegisterReq struct {
	RequestedID uint16 // 0 for auto-assignment by server
}

// RegisterResp provides the station with its identity and unique Walsh code.
type RegisterResp struct {
	AssignedID    uint16
	CodeLen       uint16 // M (length of Walsh vector)
	TotalStations uint16 // N (total stations expected in simulation)
	WalshCode     []int8 // M elements (+1 or -1)
}

// RoundStart notifies all stations that a new round has begun.
type RoundStart struct {
	RoundSeq uint32
}

// ChipTx is sent by each station containing its modulated chips for the round.
type ChipTx struct {
	StationID uint16
	RoundSeq  uint32
	Chips     []int8 // Length M; elements are +1, -1, or 0 (if silent)
}

// ChannelBroadcast carries the composite superposition of all stations' chips.
type ChannelBroadcast struct {
	RoundSeq  uint32
	Composite []int16 // Length M; algebraic sum of all chips in the round
}

// ─── Wire Framing Helpers ─────────────────────────────────────────────────────
// Frame layout:
// [1 byte Opcode] [2 bytes BigEndian PayloadLength] [Payload bytes...]

// WriteMsg writes a framed message to w.
func WriteMsg(w io.Writer, opcode byte, payload []byte) error {
	buf := make([]byte, 3+len(payload))
	buf[0] = opcode
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(payload)))
	copy(buf[3:], payload)
	_, err := w.Write(buf)
	return err
}

// ReadMsg reads a framed message from r.
func ReadMsg(r io.Reader) (byte, []byte, error) {
	header := make([]byte, 3)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}
	opcode := header[0]
	length := binary.BigEndian.Uint16(header[1:3])
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return 0, nil, err
		}
	}
	return opcode, payload, nil
}

// ─── Serialization & Deserialization ──────────────────────────────────────────

// WriteRegisterReq encodes and sends a RegisterReq.
func WriteRegisterReq(w io.Writer, req RegisterReq) error {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload[0:2], req.RequestedID)
	return WriteMsg(w, OpRegisterReq, payload)
}

// ReadRegisterReq decodes a RegisterReq payload.
func ReadRegisterReq(payload []byte) (RegisterReq, error) {
	if len(payload) < 2 {
		return RegisterReq{}, fmt.Errorf("invalid RegisterReq payload length: %d", len(payload))
	}
	return RegisterReq{
		RequestedID: binary.BigEndian.Uint16(payload[0:2]),
	}, nil
}

// WriteRegisterResp encodes and sends a RegisterResp.
func WriteRegisterResp(w io.Writer, resp RegisterResp) error {
	payload := make([]byte, 6+len(resp.WalshCode))
	binary.BigEndian.PutUint16(payload[0:2], resp.AssignedID)
	binary.BigEndian.PutUint16(payload[2:4], resp.CodeLen)
	binary.BigEndian.PutUint16(payload[4:6], resp.TotalStations)
	for i, c := range resp.WalshCode {
		payload[6+i] = byte(c)
	}
	return WriteMsg(w, OpRegisterResp, payload)
}

// ReadRegisterResp decodes a RegisterResp payload.
func ReadRegisterResp(payload []byte) (RegisterResp, error) {
	if len(payload) < 6 {
		return RegisterResp{}, fmt.Errorf("invalid RegisterResp length: %d", len(payload))
	}
	assignedID := binary.BigEndian.Uint16(payload[0:2])
	codeLen := binary.BigEndian.Uint16(payload[2:4])
	totalStations := binary.BigEndian.Uint16(payload[4:6])

	if len(payload) < int(6+codeLen) {
		return RegisterResp{}, fmt.Errorf("truncated WalshCode in RegisterResp: expected %d bytes, got %d", codeLen, len(payload)-6)
	}

	walshCode := make([]int8, codeLen)
	for i := 0; i < int(codeLen); i++ {
		walshCode[i] = int8(payload[6+i])
	}

	return RegisterResp{
		AssignedID:    assignedID,
		CodeLen:       codeLen,
		TotalStations: totalStations,
		WalshCode:     walshCode,
	}, nil
}

// WriteRoundStart encodes and sends an OpRoundStart notification.
func WriteRoundStart(w io.Writer, start RoundStart) error {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload[0:4], start.RoundSeq)
	return WriteMsg(w, OpRoundStart, payload)
}

// ReadRoundStart decodes a RoundStart payload.
func ReadRoundStart(payload []byte) (RoundStart, error) {
	if len(payload) < 4 {
		return RoundStart{}, fmt.Errorf("invalid RoundStart length: %d", len(payload))
	}
	return RoundStart{
		RoundSeq: binary.BigEndian.Uint32(payload[0:4]),
	}, nil
}

// WriteChipTx encodes and sends a ChipTx.
func WriteChipTx(w io.Writer, tx ChipTx) error {
	payload := make([]byte, 6+len(tx.Chips))
	binary.BigEndian.PutUint16(payload[0:2], tx.StationID)
	binary.BigEndian.PutUint32(payload[2:6], tx.RoundSeq)
	for i, c := range tx.Chips {
		payload[6+i] = byte(c)
	}
	return WriteMsg(w, OpChipTx, payload)
}

// ReadChipTx decodes a ChipTx payload.
func ReadChipTx(payload []byte) (ChipTx, error) {
	if len(payload) < 6 {
		return ChipTx{}, fmt.Errorf("invalid ChipTx length: %d", len(payload))
	}
	stationID := binary.BigEndian.Uint16(payload[0:2])
	roundSeq := binary.BigEndian.Uint32(payload[2:6])
	chipBytes := payload[6:]
	chips := make([]int8, len(chipBytes))
	for i, b := range chipBytes {
		chips[i] = int8(b)
	}
	return ChipTx{
		StationID: stationID,
		RoundSeq:  roundSeq,
		Chips:     chips,
	}, nil
}

// WriteChannelBroadcast encodes and sends a ChannelBroadcast composite vector.
func WriteChannelBroadcast(w io.Writer, bcast ChannelBroadcast) error {
	payload := make([]byte, 4+2*len(bcast.Composite))
	binary.BigEndian.PutUint32(payload[0:4], bcast.RoundSeq)
	offset := 4
	for _, val := range bcast.Composite {
		binary.BigEndian.PutUint16(payload[offset:offset+2], uint16(val))
		offset += 2
	}
	return WriteMsg(w, OpChannelBroadcast, payload)
}

// ReadChannelBroadcast decodes a ChannelBroadcast payload.
func ReadChannelBroadcast(payload []byte) (ChannelBroadcast, error) {
	if len(payload) < 4 {
		return ChannelBroadcast{}, fmt.Errorf("invalid ChannelBroadcast length: %d", len(payload))
	}
	roundSeq := binary.BigEndian.Uint32(payload[0:4])
	rawValues := payload[4:]
	count := len(rawValues) / 2
	composite := make([]int16, count)
	for i := 0; i < count; i++ {
		composite[i] = int16(binary.BigEndian.Uint16(rawValues[i*2 : i*2+2]))
	}
	return ChannelBroadcast{
		RoundSeq:  roundSeq,
		Composite: composite,
	}, nil
}
