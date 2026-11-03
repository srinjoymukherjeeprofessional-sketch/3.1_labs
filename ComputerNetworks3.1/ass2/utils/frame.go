// Package utils provides Data Link Layer simulation primitives for Assignment 2.
//
// # Frame Wire Layout (15-byte header + payload + 4-byte FCS trailer)
//
//	[0:6]   Source MAC address (6 bytes)
//	[6:12]  Destination MAC address (6 bytes)
//	[12:14] Payload length — actual unpadded byte count (big-endian uint16)
//	[14]    Control byte:
//	          Bits 7-6: FrameType  (00=Data, 01=ACK, 10=NAK, 11=EOF)
//	          Bits 5-0: SeqNum     (range 0–63, 6-bit field)
//	[15 : 15+wirePayloadLen]  Payload (zero-padded to MinPayloadSize for Data frames)
//	[last 4 bytes]            FCS trailer (big-endian uint32)
//
// The FCS algorithm (CRC32 or Internet Checksum) is NOT embedded in the frame.
// Sender and receiver agree on the algorithm at startup via explicit input.
package utils

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ── Frame type constants (packed into bits 7-6 of the control byte) ───────────

const (
	TypeData   byte = 0x00 // Data frame carrying a payload chunk
	TypeACK    byte = 0x01 // Acknowledgement — positive confirmation of receipt
	TypeNAK    byte = 0x02 // Negative Acknowledgement — request for retransmission (SR)
	TypeEOF    byte = 0x03 // End-of-file sentinel — sender signals no more data
	TypeConfig byte = 0x04 // Channel config handshake — syncs sender channel params to receiver
)

// ── Size constants (per assignment spec Fig. 1) ────────────────────────────────

const (
	FixedPayloadSize = 64 // Fixed payload size per assignment specification
	HeaderSize       = 15 // 6 (SrcMAC) + 6 (DstMAC) + 2 (Length) + 1 (Control)
	TrailerSize      = 4  // 4-byte FCS
	DataFrameSize    = HeaderSize + FixedPayloadSize + TrailerSize // 83 bytes
	ControlFrameSize = HeaderSize + TrailerSize                    // 19 bytes
	MinPayloadSize   = 46   // Ethernet minimum; Data frames below this are zero-padded
	MaxPayloadSize   = 1500 // Ethernet MTU; payloads above this are rejected
)

// ── Header and Frame structs ───────────────────────────────────────────────────

// Header holds the decoded 15-byte frame header fields.
type Header struct {
	SrcMAC    [6]byte // Source MAC address
	DstMAC    [6]byte // Destination MAC address
	Length    uint16  // True (unpadded) payload length stored in the header
	FrameType byte    // TypeData / TypeACK / TypeNAK / TypeEOF
	SeqNum    uint8   // Sequence number (6-bit field → range 0–63)
}

// Frame represents a complete Data Link Layer frame (Header + Payload + FCS).
type Frame struct {
	Header  Header
	Payload []byte // Decoded application payload (unpadded, nil for ACK/NAK/EOF)
	FCS     uint32 // FCS value read from the trailer (for inspection / logging)
}

// ── Constructors ───────────────────────────────────────────────────────────────

// NewDataFrame constructs a Data frame wrapping the given payload bytes.
// seqNum is masked to 6 bits (0–63) automatically.
func NewDataFrame(src, dst [6]byte, seqNum uint8, payload []byte) *Frame {
	return &Frame{
		Header: Header{
			SrcMAC:    src,
			DstMAC:    dst,
			Length:    uint16(len(payload)),
			FrameType: TypeData,
			SeqNum:    seqNum & 0x3F,
		},
		Payload: payload,
	}
}

// NewACKFrame constructs a positive acknowledgement for the given sequence number.
func NewACKFrame(src, dst [6]byte, ackNum uint8) *Frame {
	return &Frame{
		Header: Header{
			SrcMAC:    src,
			DstMAC:    dst,
			FrameType: TypeACK,
			SeqNum:    ackNum & 0x3F,
		},
	}
}

// NewNAKFrame constructs a negative acknowledgement requesting retransmission
// of the frame with the given sequence number (used in SR with NAK enabled).
func NewNAKFrame(src, dst [6]byte, nakNum uint8) *Frame {
	return &Frame{
		Header: Header{
			SrcMAC:    src,
			DstMAC:    dst,
			FrameType: TypeNAK,
			SeqNum:    nakNum & 0x3F,
		},
	}
}

// NewEOFFrame constructs an EOF sentinel frame. The sender transmits this after
// all data frames are acknowledged; the receiver sends an ACK back and closes.
func NewEOFFrame(src, dst [6]byte) *Frame {
	return &Frame{
		Header: Header{
			SrcMAC:    src,
			DstMAC:    dst,
			FrameType: TypeEOF,
			Length:    0,
		},
		Payload: make([]byte, FixedPayloadSize),
	}
}

// NewConfigFrame constructs an initial handshake frame carrying sender's channel settings.
// This allows the receiver to mirror the exact same channel impairments on the ACK path.
func NewConfigFrame(src, dst [6]byte, loss, delay float64, delayDur time.Duration, corrupt float64) *Frame {
	payload := make([]byte, FixedPayloadSize)
	msg := fmt.Sprintf("loss=%.4f;delay=%.4f;dur=%d;corrupt=%.4f", loss, delay, delayDur.Milliseconds(), corrupt)
	copy(payload, msg)
	return &Frame{
		Header: Header{
			SrcMAC:    src,
			DstMAC:    dst,
			Length:    uint16(len(msg)),
			FrameType: TypeConfig,
		},
		Payload: payload,
	}
}

// ParseConfigFrame extracts channel impairment parameters from a TypeConfig frame payload.
func ParseConfigFrame(f *Frame) (loss, delay float64, delayDur time.Duration, corrupt float64, err error) {
	if f == nil || len(f.Payload) == 0 {
		return 0, 0, 0, 0, errors.New("empty config frame")
	}
	length := int(f.Header.Length)
	if length > len(f.Payload) {
		length = len(f.Payload)
	}
	str := string(f.Payload[:length])
	parts := strings.Split(str, ";")
	m := map[string]string{}
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	loss, _ = strconv.ParseFloat(m["loss"], 64)
	delay, _ = strconv.ParseFloat(m["delay"], 64)
	durMs, _ := strconv.Atoi(m["dur"])
	corrupt, _ = strconv.ParseFloat(m["corrupt"], 64)
	return loss, delay, time.Duration(durMs) * time.Millisecond, corrupt, nil
}

// ── FCS helper ────────────────────────────────────────────────────────────────

// computeFCS returns a 4-byte FCS for data using the chosen algorithm.
// fcsAlgo must be "crc32" (default) or "checksum" (16-bit Internet Checksum,
// zero-extended to 32 bits).
func computeFCS(data []byte, fcsAlgo string) uint32 {
	if fcsAlgo == "checksum" {
		return uint32(Checksum(data))
	}
	return CRC32(data)
}

// ── Serialization ─────────────────────────────────────────────────────────────

// Serialize converts the Frame into a raw byte slice suitable for transmission.
func (frame *Frame) Serialize(fcsAlgo string) ([]byte, error) {
	if len(frame.Payload) > MaxPayloadSize {
		return nil, fmt.Errorf("payload %d B exceeds MTU %d B", len(frame.Payload), MaxPayloadSize)
	}

	// Fixed-size wire framing (Option A):
	// Data, EOF, and Config frames from Sender are always DataFrameSize (83 bytes total).
	// Control frames (ACK, NAK) from Receiver are always ControlFrameSize (19 bytes total).
	wireLen := 0
	if frame.Header.FrameType == TypeData || frame.Header.FrameType == TypeEOF || frame.Header.FrameType == TypeConfig {
		wireLen = FixedPayloadSize
	}

	// Allocate and zero-fill (make guarantees zeros → free padding)
	total := HeaderSize + wireLen + TrailerSize
	buf := make([]byte, total)

	// [0:6]  Source MAC
	copy(buf[0:6], frame.Header.SrcMAC[:])
	// [6:12] Destination MAC
	copy(buf[6:12], frame.Header.DstMAC[:])
	// [12:14] True payload length (big-endian)
	binary.BigEndian.PutUint16(buf[12:14], frame.Header.Length)
	// [14]   Control byte
	if frame.Header.FrameType == TypeConfig {
		buf[14] = 0xFE
	} else {
		buf[14] = (frame.Header.FrameType&0x03)<<6 | (frame.Header.SeqNum & 0x3F)
	}
	// [15 : 15+len(Payload)]  Payload bytes (rest of wireLen is already zero)
	copy(buf[15:], frame.Payload)

	// Compute FCS over header + wire-padded payload, append as 4-byte trailer
	fcsVal := computeFCS(buf[:HeaderSize+wireLen], fcsAlgo)
	frame.FCS = fcsVal
	binary.BigEndian.PutUint32(buf[total-TrailerSize:], fcsVal)

	return buf, nil
}

// ── Deserialization ───────────────────────────────────────────────────────────

// Deserialize parses a raw byte slice back into a Frame and verifies the FCS.
func Deserialize(raw []byte, fcsAlgo string) (*Frame, error) {
	if len(raw) < HeaderSize+TrailerSize {
		return nil, errors.New("frame too short: smaller than header + trailer minimum")
	}

	// Parse header fields
	var h Header
	copy(h.SrcMAC[:], raw[0:6])
	copy(h.DstMAC[:], raw[6:12])
	h.Length = binary.BigEndian.Uint16(raw[12:14])
	ctrl := raw[14]
	if ctrl == 0xFE {
		h.FrameType = TypeConfig
		h.SeqNum = 0
	} else {
		h.FrameType = (ctrl >> 6) & 0x03
		h.SeqNum = ctrl & 0x3F
	}

	// Verify FCS: recompute over everything except the 4-byte trailer
	rxFCS := binary.BigEndian.Uint32(raw[len(raw)-TrailerSize:])
	exFCS := computeFCS(raw[:len(raw)-TrailerSize], fcsAlgo)
	if rxFCS != exFCS {
		return nil, fmt.Errorf("FCS mismatch (corrupted): want 0x%08X got 0x%08X", exFCS, rxFCS)
	}

	// Extract unpadded payload using the true length stored in the header
	var payload []byte
	if h.Length > 0 {
		end := HeaderSize + int(h.Length)
		if end > len(raw)-TrailerSize {
			return nil, errors.New("header.Length exceeds available payload region")
		}
		payload = append([]byte(nil), raw[HeaderSize:end]...)
	}

	return &Frame{Header: h, Payload: payload, FCS: rxFCS}, nil
}
