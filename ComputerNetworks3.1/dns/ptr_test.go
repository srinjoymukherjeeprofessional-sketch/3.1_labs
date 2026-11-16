package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPTRAnswers(t *testing.T) {
	loaded, err := loadRecords("records.json")
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{records: loaded}
	for _, tc := range []struct {
		name   string
		target string
		rcode  uint8
	}{
		{"1.0.0.127.in-addr.arpa.", "example.test.", 0},
		{"10.2.0.192.IN-ADDR.ARPA.", "api.example.test.", 0},
		{"example.test.", "", 0}, // Existing name without a PTR record.
		{"2.0.0.127.in-addr.arpa.", "", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			question := Question{Name: tc.name, Type: 12, Class: 1}
			request := Message{
				Header:    Header{ID: 1234, Flags: Flags{RD: true}},
				Questions: []Question{question},
			}
			packet, err := request.Marshal()
			if err != nil {
				t.Fatal(err)
			}
			packet, err = server.handleQuery(packet)
			if err != nil {
				t.Fatal(err)
			}
			response, err := ParseMessage(packet)
			if err != nil {
				t.Fatal(err)
			}
			if response.Header.Flags.RCode != tc.rcode {
				t.Fatalf("got RCode %d, want %d", response.Header.Flags.RCode, tc.rcode)
			}
			if tc.target == "" {
				if len(response.Answers) != 0 {
					t.Fatal("expected no answers")
				}
				return
			}
			if len(response.Answers) != 1 {
				t.Fatalf("got %d answers", len(response.Answers))
			}
			answer := response.Answers[0]
			if answer.Name != tc.name || answer.Type != 12 || answer.Class != 1 || answer.TTL != 60 {
				t.Fatalf("unexpected answer: %+v", answer)
			}
			name, end, err := readName(answer.Data, 0)
			if err != nil || name != tc.target || end != len(answer.Data) {
				t.Fatalf("got target %q, end %d, error %v", name, end, err)
			}
		})
	}
}

func TestLoadPTRRecords(t *testing.T) {
	for _, tc := range []struct {
		target string
		valid  bool
	}{
		{"Example.test", true},
		{".", true},
		{"bad..test", false},
		{" ", false},
		{strings.Repeat("a", 64) + ".test", false},
		{strings.Repeat(strings.Repeat("a", 63)+".", 4), false},
	} {
		path := filepath.Join(t.TempDir(), "records.json")
		data := `{"1.0.0.127.in-addr.arpa": {"PTR": "` + tc.target + `"}}`
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		loaded, err := loadRecords(path)
		if !tc.valid {
			if err == nil {
				t.Fatalf("accepted invalid PTR target %q", tc.target)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		want := strings.TrimSuffix(tc.target, ".") + "."
		if got := loaded["1.0.0.127.in-addr.arpa."].PTR; got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
