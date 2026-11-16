package main

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Flags holds the fields packed into the DNS header's 16-bit flags word.
type Flags struct {
	QR     bool  // Response rather than query
	Opcode uint8 // Operation (0 = standard query)
	AA     bool  // Authoritative answer
	TC     bool  // Truncated message
	RD     bool  // Recursion desired
	RA     bool  // Recursion available
	Z      bool  // Reserved; normally false
	AD     bool  // Authenticated data
	CD     bool  // Checking disabled
	RCode  uint8 // Header response code (0 = success, 3 = NXDOMAIN)
}

type Header struct {
	ID      uint16
	Flags   Flags
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

type Question struct {
	Name  string
	Type  uint16
	Class uint16
}

type ResourceRecord struct {
	Name  string
	Type  uint16
	Class uint16
	TTL   uint32
	// Data is raw RDATA. A and AAAA contain address bytes. For record types
	// containing compressed names, decode those names before relocating RDATA.
	Data []byte
}

type Message struct {
	Header     Header
	Questions  []Question
	Answers    []ResourceRecord
	Authority  []ResourceRecord
	Additional []ResourceRecord
}

func parseFlags(word uint16) Flags {
	return Flags{
		QR: word&(1<<15) != 0, Opcode: uint8(word >> 11 & 15),
		AA: word&(1<<10) != 0, TC: word&(1<<9) != 0,
		RD: word&(1<<8) != 0, RA: word&(1<<7) != 0,
		Z: word&(1<<6) != 0, AD: word&(1<<5) != 0,
		CD: word&(1<<4) != 0, RCode: uint8(word & 15),
	}
}

func (f Flags) word() (uint16, error) {
	if f.Opcode > 15 || f.RCode > 15 {
		return 0, fmt.Errorf("opcode and header response code must fit in 4 bits")
	}
	word := uint16(f.Opcode)<<11 | uint16(f.RCode)
	for _, field := range []struct {
		set bool
		bit uint
	}{
		{f.QR, 15}, {f.AA, 10}, {f.TC, 9}, {f.RD, 8},
		{f.RA, 7}, {f.Z, 6}, {f.AD, 5}, {f.CD, 4},
	} {
		if field.set {
			word |= 1 << field.bit
		}
	}
	return word, nil
}

// ParseMessage decodes all five sections, including compressed owner names.
// Record-specific RDATA stays as bytes; EDNS extensions aren't interpreted.
func ParseMessage(packet []byte) (*Message, error) {
	if len(packet) < 12 {
		return nil, fmt.Errorf("packet shorter than DNS header")
	}
	m := &Message{Header: Header{
		ID:      binary.BigEndian.Uint16(packet[:2]),
		Flags:   parseFlags(binary.BigEndian.Uint16(packet[2:4])),
		QDCount: binary.BigEndian.Uint16(packet[4:6]),
		ANCount: binary.BigEndian.Uint16(packet[6:8]),
		NSCount: binary.BigEndian.Uint16(packet[8:10]),
		ARCount: binary.BigEndian.Uint16(packet[10:12]),
	}}
	offset := 12
	for i := 0; i < int(m.Header.QDCount); i++ {
		name, next, err := readName(packet, offset)
		if err != nil {
			return nil, fmt.Errorf("question %d: %w", i, err)
		}
		if next+4 > len(packet) {
			return nil, fmt.Errorf("incomplete question")
		}
		m.Questions = append(m.Questions, Question{
			Name: name, Type: binary.BigEndian.Uint16(packet[next : next+2]),
			Class: binary.BigEndian.Uint16(packet[next+2 : next+4]),
		})
		offset = next + 4
	}
	for _, section := range []struct {
		count   uint16
		records *[]ResourceRecord
	}{
		{m.Header.ANCount, &m.Answers},
		{m.Header.NSCount, &m.Authority},
		{m.Header.ARCount, &m.Additional},
	} {
		for i := 0; i < int(section.count); i++ {
			name, next, err := readName(packet, offset)
			if err != nil {
				return nil, fmt.Errorf("resource record: %w", err)
			}
			if next+10 > len(packet) {
				return nil, fmt.Errorf("incomplete resource record")
			}
			end := next + 10 + int(binary.BigEndian.Uint16(packet[next+8:next+10]))
			if end > len(packet) {
				return nil, fmt.Errorf("incomplete record data")
			}
			*section.records = append(*section.records, ResourceRecord{
				Name: name, Type: binary.BigEndian.Uint16(packet[next : next+2]),
				Class: binary.BigEndian.Uint16(packet[next+2 : next+4]),
				TTL:   binary.BigEndian.Uint32(packet[next+4 : next+8]),
				Data:  append([]byte(nil), packet[next+10:end]...),
			})
			offset = end
		}
	}
	if offset != len(packet) {
		return nil, fmt.Errorf("unexpected bytes after DNS sections")
	}
	return m, nil
}

// readName follows compression pointers while returning the position after
// the original name, so the caller can continue reading the enclosing record.
func readName(packet []byte, offset int) (string, int, error) {
	var labels []string
	next := -1
	length := 1 // Include the terminating root byte.
	visited := make(map[int]bool)
	for {
		if offset < 0 || offset >= len(packet) {
			return "", 0, fmt.Errorf("incomplete name")
		}
		if visited[offset] {
			return "", 0, fmt.Errorf("name compression loop")
		}
		visited[offset] = true
		size := int(packet[offset])
		switch size & 0xc0 {
		case 0xc0:
			if offset+1 >= len(packet) {
				return "", 0, fmt.Errorf("incomplete compression pointer")
			}
			if next == -1 {
				next = offset + 2
			}
			offset = (size&0x3f)<<8 | int(packet[offset+1])
			continue
		case 0:
			// A normal label, at most 63 bytes.
		default:
			return "", 0, fmt.Errorf("invalid label encoding")
		}
		offset++
		if size == 0 {
			if next == -1 {
				next = offset
			}
			if len(labels) == 0 {
				return ".", next, nil
			}
			return strings.Join(labels, ".") + ".", next, nil
		}
		if offset+size > len(packet) {
			return "", 0, fmt.Errorf("incomplete label")
		}
		length += size + 1
		if length > 255 {
			return "", 0, fmt.Errorf("name too long")
		}
		labels = append(labels, string(packet[offset:offset+size]))
		offset += size
	}
}

// Marshal derives section counts from the slices rather than Header's counts.
// Raw RDATA containing packet-relative compression pointers must not be copied
// from another packet without first decoding and re-encoding those names.
func (m *Message) Marshal() ([]byte, error) {
	flags, err := m.Header.Flags.word()
	if err != nil {
		return nil, err
	}
	packet := binary.BigEndian.AppendUint16(nil, m.Header.ID)
	packet = binary.BigEndian.AppendUint16(packet, flags)
	for _, count := range []int{len(m.Questions), len(m.Answers), len(m.Authority), len(m.Additional)} {
		if count > 65535 {
			return nil, fmt.Errorf("too many entries in DNS section")
		}
		packet = binary.BigEndian.AppendUint16(packet, uint16(count))
	}
	names := make(map[string]uint16)
	for _, q := range m.Questions {
		packet, err = appendName(packet, q.Name, names)
		if err != nil {
			return nil, err
		}
		packet = binary.BigEndian.AppendUint16(packet, q.Type)
		packet = binary.BigEndian.AppendUint16(packet, q.Class)
	}
	for _, section := range [][]ResourceRecord{m.Answers, m.Authority, m.Additional} {
		for _, rr := range section {
			packet, err = appendName(packet, rr.Name, names)
			if err != nil {
				return nil, err
			}
			if len(rr.Data) > 65535 {
				return nil, fmt.Errorf("record data too long")
			}
			packet = binary.BigEndian.AppendUint16(packet, rr.Type)
			packet = binary.BigEndian.AppendUint16(packet, rr.Class)
			packet = binary.BigEndian.AppendUint32(packet, rr.TTL)
			packet = binary.BigEndian.AppendUint16(packet, uint16(len(rr.Data)))
			packet = append(packet, rr.Data...)
		}
	}
	if len(packet) > 65535 {
		return nil, fmt.Errorf("DNS message too long")
	}
	return packet, nil
}

func appendName(packet []byte, name string, names map[string]uint16) ([]byte, error) {
	if name == "." {
		return append(packet, 0), nil
	}
	name = strings.TrimSuffix(name, ".") + "."
	if position, exists := names[name]; exists {
		return binary.BigEndian.AppendUint16(packet, 0xc000|position), nil
	}
	start := len(packet)
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, fmt.Errorf("invalid label in name %q", name)
		}
		packet = append(packet, byte(len(label)))
		packet = append(packet, label...)
	}
	packet = append(packet, 0)
	if len(packet)-start > 255 {
		return nil, fmt.Errorf("name too long")
	}
	if start < 1<<14 {
		names[name] = uint16(start)
	}
	return packet, nil
}
