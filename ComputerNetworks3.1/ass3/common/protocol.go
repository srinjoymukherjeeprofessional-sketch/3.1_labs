package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

// ─── Protocol Opcodes ──────────────────────────────────────────────────────────

const (
	OpSenseReq    byte = 0x01 // Station -> Server: Sense channel
	OpRespIdle    byte = 0x02 // Server -> Station: Channel is IDLE
	OpRespBusy    byte = 0x03 // Server -> Station: Channel is BUSY
	OpTxStartReq  byte = 0x10 // Station -> Server: Start transmission
	OpTxEndReq    byte = 0x11 // Station -> Server: End transmission
	OpRespSuccess byte = 0x20 // Server -> Station: Frame transmitted successfully
	OpRespJam     byte = 0x21 // Server -> Station: Collision occurred (Jam/Corrupted)
)

// ─── Default Physical & MAC Constants ─────────────────────────────────────────

const (
	DefaultTp          = 5 * time.Millisecond  // One-way propagation delay (vulnerability window)
	DefaultTfr         = 10 * time.Millisecond // Frame transmission duration (Tfr >= 2*Tp)
	DefaultSlotTime    = 2 * DefaultTp         // Slot time for BEB and p-persistent (10ms)
	DefaultIFG         = 1 * time.Millisecond  // Inter-frame gap
	DefaultPayloadSize = 64                    // Bytes per frame
	MaxBEBAttempts     = 16                    // Abort after 16 consecutive collisions
	MaxBEBExponent     = 10                    // Backoff window capped at 2^10 - 1 = 1023
)

// ─── Protocol Message Structs ─────────────────────────────────────────────────

// SenseReq represents a carrier sense query from a station.
type SenseReq struct {
	StationID uint16
}

// TxStartReq represents the beginning of a frame transmission.
type TxStartReq struct {
	StationID  uint16
	FrameSeq   uint32
	PayloadLen uint16
	TfrMs      uint32 // Planned transmission duration in milliseconds
}

// TxEndReq represents the completion of a frame transmission.
type TxEndReq struct {
	StationID uint16
	FrameSeq  uint32
}

// TxResultResp represents the server's verdict for a transmitted frame.
type TxResultResp struct {
	Opcode         byte   // OpRespSuccess or OpRespJam
	FrameSeq       uint32
	CollisionCount uint32
}

// ─── Wire Framing Helpers ─────────────────────────────────────────────────────
// Each frame on the wire is formatted as:
// [1 byte Opcode] [2 bytes BigEndian PayloadLength] [N bytes Payload]

// WriteMsg writes an opcode and payload slice to w.
func WriteMsg(w io.Writer, opcode byte, payload []byte) error {
	buf := make([]byte, 3+len(payload))
	buf[0] = opcode
	binary.BigEndian.PutUint16(buf[1:3], uint16(len(payload)))
	copy(buf[3:], payload)
	_, err := w.Write(buf)
	return err
}

// ReadMsg reads an opcode and payload slice from r.
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

// ─── Typed Serialization / Deserialization ────────────────────────────────────

// WriteSenseReq encodes and sends a SenseReq.
func WriteSenseReq(w io.Writer, req SenseReq) error {
	payload := make([]byte, 2)
	binary.BigEndian.PutUint16(payload, req.StationID)
	return WriteMsg(w, OpSenseReq, payload)
}

// ReadSenseReq decodes a SenseReq payload.
func ReadSenseReq(payload []byte) (SenseReq, error) {
	if len(payload) < 2 {
		return SenseReq{}, fmt.Errorf("invalid SenseReq payload length: %d", len(payload))
	}
	return SenseReq{
		StationID: binary.BigEndian.Uint16(payload[0:2]),
	}, nil
}

// WriteSenseResp writes either OpRespIdle or OpRespBusy.
func WriteSenseResp(w io.Writer, isIdle bool) error {
	op := OpRespBusy
	if isIdle {
		op = OpRespIdle
	}
	return WriteMsg(w, op, nil)
}

// WriteTxStartReq encodes and sends a TxStartReq.
func WriteTxStartReq(w io.Writer, req TxStartReq) error {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint16(payload[0:2], req.StationID)
	binary.BigEndian.PutUint32(payload[2:6], req.FrameSeq)
	binary.BigEndian.PutUint16(payload[6:8], req.PayloadLen)
	binary.BigEndian.PutUint32(payload[8:12], req.TfrMs)
	return WriteMsg(w, OpTxStartReq, payload)
}

// ReadTxStartReq decodes a TxStartReq payload.
func ReadTxStartReq(payload []byte) (TxStartReq, error) {
	if len(payload) < 12 {
		return TxStartReq{}, fmt.Errorf("invalid TxStartReq payload length: %d", len(payload))
	}
	return TxStartReq{
		StationID:  binary.BigEndian.Uint16(payload[0:2]),
		FrameSeq:   binary.BigEndian.Uint32(payload[2:6]),
		PayloadLen: binary.BigEndian.Uint16(payload[6:8]),
		TfrMs:      binary.BigEndian.Uint32(payload[8:12]),
	}, nil
}

// WriteTxEndReq encodes and sends a TxEndReq.
func WriteTxEndReq(w io.Writer, req TxEndReq) error {
	payload := make([]byte, 6)
	binary.BigEndian.PutUint16(payload[0:2], req.StationID)
	binary.BigEndian.PutUint32(payload[2:6], req.FrameSeq)
	return WriteMsg(w, OpTxEndReq, payload)
}

// ReadTxEndReq decodes a TxEndReq payload.
func ReadTxEndReq(payload []byte) (TxEndReq, error) {
	if len(payload) < 6 {
		return TxEndReq{}, fmt.Errorf("invalid TxEndReq payload length: %d", len(payload))
	}
	return TxEndReq{
		StationID: binary.BigEndian.Uint16(payload[0:2]),
		FrameSeq:  binary.BigEndian.Uint32(payload[2:6]),
	}, nil
}

// WriteTxResultResp encodes and sends a TxResultResp (Success or Jam).
func WriteTxResultResp(w io.Writer, resp TxResultResp) error {
	payload := make([]byte, 8)
	binary.BigEndian.PutUint32(payload[0:4], resp.FrameSeq)
	binary.BigEndian.PutUint32(payload[4:8], resp.CollisionCount)
	return WriteMsg(w, resp.Opcode, payload)
}

// ReadTxResultResp decodes a TxResultResp from opcode and payload.
func ReadTxResultResp(opcode byte, payload []byte) (TxResultResp, error) {
	if len(payload) < 8 {
		return TxResultResp{}, fmt.Errorf("invalid TxResultResp payload length: %d", len(payload))
	}
	return TxResultResp{
		Opcode:         opcode,
		FrameSeq:       binary.BigEndian.Uint32(payload[0:4]),
		CollisionCount: binary.BigEndian.Uint32(payload[4:8]),
	}, nil
}
