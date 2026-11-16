package main

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestIndividualFlags(t *testing.T) {
	// Independent wire values catch fields being assigned to the wrong bit.
	for _, tc := range []struct {
		word uint16
		want Flags
	}{
		{0x8000, Flags{QR: true}},
		{0x7800, Flags{Opcode: 15}},
		{0x0400, Flags{AA: true}},
		{0x0200, Flags{TC: true}},
		{0x0100, Flags{RD: true}},
		{0x0080, Flags{RA: true}},
		{0x0040, Flags{Z: true}},
		{0x0020, Flags{AD: true}},
		{0x0010, Flags{CD: true}},
		{0x0003, Flags{RCode: 3}},
	} {
		packet := make([]byte, 12)
		binary.BigEndian.PutUint16(packet[2:4], tc.word)
		message, err := ParseMessage(packet)
		if err != nil {
			t.Fatal(err)
		}
		if message.Header.Flags != tc.want {
			t.Fatalf("flags %04x: got %+v, want %+v", tc.word, message.Header.Flags, tc.want)
		}
		encoded, err := message.Marshal()
		if err != nil || !bytes.Equal(encoded, packet) {
			t.Fatalf("flags %04x: encoded %x, error %v", tc.word, encoded, err)
		}
	}
}

func TestParseAllSections(t *testing.T) {
	// One question, compressed A answer, TXT authority record, and OPT record.
	packet := []byte{
		0x12, 0x34, 0x85, 0x00, 0, 1, 0, 1, 0, 1, 0, 1,
		1, 'a', 4, 't', 'e', 's', 't', 0, 0, 1, 0, 1,
		0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 127, 0, 0, 1,
		0xc0, 0x0c, 0, 16, 0, 1, 0, 0, 0, 30, 0, 3, 2, 'o', 'k',
		0, 0, 41, 0x10, 0, 0, 0, 0, 0, 0, 0,
	}
	m, err := ParseMessage(packet)
	if err != nil {
		t.Fatal(err)
	}
	if m.Header.ID != 0x1234 || !m.Header.Flags.QR || !m.Header.Flags.AA || !m.Header.Flags.RD {
		t.Fatalf("unexpected header: %+v", m.Header)
	}
	if !reflect.DeepEqual(m.Questions, []Question{{Name: "a.test.", Type: 1, Class: 1}}) {
		t.Fatalf("unexpected questions: %+v", m.Questions)
	}
	if len(m.Answers) != 1 || m.Answers[0].Name != "a.test." || m.Answers[0].TTL != 60 || !bytes.Equal(m.Answers[0].Data, []byte{127, 0, 0, 1}) {
		t.Fatalf("unexpected answers: %+v", m.Answers)
	}
	if len(m.Authority) != 1 || m.Authority[0].Type != 16 || !bytes.Equal(m.Authority[0].Data, []byte{2, 'o', 'k'}) {
		t.Fatalf("unexpected authority: %+v", m.Authority)
	}
	if len(m.Additional) != 1 || m.Additional[0].Name != "." || m.Additional[0].Type != 41 || m.Additional[0].Class != 4096 {
		t.Fatalf("unexpected additional: %+v", m.Additional)
	}
	// Counts for outgoing messages come from the slices.
	m.Header.QDCount = 99
	encoded, err := m.Marshal()
	if err != nil || !bytes.Equal(encoded, packet) {
		t.Fatalf("encoded %x, want %x, error %v", encoded, packet, err)
	}
	for end := 0; end < len(packet); end++ {
		if _, err := ParseMessage(packet[:end]); err == nil {
			t.Fatalf("accepted truncated packet of length %d", end)
		}
	}
}

func TestReadNameCompression(t *testing.T) {
	packet := []byte{4, 't', 'e', 's', 't', 0, 3, 'a', 'p', 'i', 0xc0, 0}
	name, next, err := readName(packet, 6)
	if err != nil || name != "api.test." || next != len(packet) {
		t.Fatalf("got %q, %d, %v", name, next, err)
	}
	for _, invalid := range [][]byte{
		{0xc0}, {0xc0, 0}, {0xc0, 2, 0xc0, 0}, {0xc0, 255},
		{0x40}, {0x80}, {3, 'a'},
	} {
		if _, _, err := readName(invalid, 0); err == nil {
			t.Fatalf("accepted malformed name %x", invalid)
		}
	}
}

func TestHandleQueryPreservesRD(t *testing.T) {
	for _, rd := range []bool{false, true} {
		request := Message{
			Header:    Header{ID: 42, Flags: Flags{RD: rd}},
			Questions: []Question{{Name: "missing.test.", Type: 28, Class: 1}},
		}
		packet, err := request.Marshal()
		if err != nil {
			t.Fatal(err)
		}
		server := &Server{records: map[string]addressRecords{"missing.test.": {}}}
		response, err := server.handleQuery(packet)
		if err != nil {
			t.Fatal(err)
		}
		m, err := ParseMessage(response)
		if err != nil {
			t.Fatal(err)
		}
		if m.Header.ID != 42 || !m.Header.Flags.QR || !m.Header.Flags.AA || m.Header.Flags.RD != rd || m.Header.Flags.RA {
			t.Fatalf("unexpected response: %+v", m.Header)
		}
	}
}
