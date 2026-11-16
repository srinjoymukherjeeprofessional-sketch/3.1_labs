package main

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAddressAnswers(t *testing.T) {
	loaded, err := loadRecords("records.json")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{records: loaded}

	for _, tc := range []struct {
		name  string
		qtype uint16
		ip    string
		rcode uint16
	}{
		{"example", 1, "127.0.0.1", 0},
		{"example", 28, "::1", 0},
		{"example", 15, "", 0},
		{"missing", 28, "", 5},
	} {
		t.Run(tc.name+"/"+strconv.Itoa(int(tc.qtype)), func(t *testing.T) {
			query := make([]byte, 12)
			binary.BigEndian.PutUint16(query[0:2], 1234)
			binary.BigEndian.PutUint16(query[4:6], 1)
			query = append(query, byte(len(tc.name)))
			query = append(query, tc.name...)
			query = append(query, 4, 't', 'e', 's', 't', 0, byte(tc.qtype>>8), byte(tc.qtype), 0, 1)
			response, err := server.handleQuery(query)
			if err != nil {
				t.Fatal(err)
			}
			if binary.BigEndian.Uint16(response[:2]) != 1234 || binary.BigEndian.Uint16(response[2:4])&15 != tc.rcode {
				t.Fatalf("unexpected response header: %x", response[:12])
			}
			count := binary.BigEndian.Uint16(response[6:8])
			if tc.ip == "" {
				if count != 0 || len(response) != len(query) {
					t.Fatal("expected empty answer")
				}
				return
			}
			ip := net.ParseIP(tc.ip)
			if tc.qtype == 1 {
				ip = ip.To4()
			} else {
				ip = ip.To16()
			}
			answer := response[len(query):]
			if count != 1 || len(answer) != 12+len(ip) {
				t.Fatalf("unexpected answer: %x", answer)
			}
			if binary.BigEndian.Uint16(answer[2:4]) != tc.qtype || int(binary.BigEndian.Uint16(answer[10:12])) != len(ip) || !bytes.Equal(answer[12:], ip) {
				t.Fatalf("incorrect address encoding: %x", answer)
			}
		})
	}
}

func TestLoadRecordsRejectsWrongAddressFamily(t *testing.T) {
	for _, data := range []string{
		`{"example.test": {"A": "::1"}}`,
		`{"example.test": {"AAAA": "127.0.0.1"}}`,
		`{"example.test": {"AAAA": "invalid"}}`,
	} {
		path := filepath.Join(t.TempDir(), "records.json")
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadRecords(path); err == nil {
			t.Fatalf("accepted invalid records: %s", data)
		}
	}
}
