package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

// This server is authoritative for these names.
type addressRecords struct {
	A    net.IP
	AAAA net.IP
}

var records map[string]addressRecords

func main() {
	var err error
	records, err = loadRecords("records.json")
	if err != nil {
		log.Fatal(err)
	}

	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5353}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	log.Printf("DNS server listening on %s", addr)

	buf := make([]byte, 4096)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read: %v", err)
			continue
		}

		response, err := handleQuery(buf[:n])
		if err != nil {
			log.Printf("invalid query from %s: %v", client, err)
			continue
		}

		if _, err := conn.WriteToUDP(response, client); err != nil {
			log.Printf("write to %s: %v", client, err)
		}
	}
}

// loadRecords reads each domain's A and AAAA records once at startup.
func loadRecords(path string) (map[string]addressRecords, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}

	var entries map[string]struct {
		A    string `json:"A"`
		AAAA string `json:"AAAA"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode records: %w", err)
	}
	if entries == nil {
		return nil, fmt.Errorf("records must be a JSON object")
	}

	loaded := make(map[string]addressRecords, len(entries))
	for name, entry := range entries {
		var addresses addressRecords
		if entry.A != "" {
			addresses.A = net.ParseIP(entry.A).To4()
			if addresses.A == nil {
				return nil, fmt.Errorf("record %q: invalid IPv4 address %q", name, entry.A)
			}
		}
		if entry.AAAA != "" {
			ip := net.ParseIP(entry.AAAA)
			if ip == nil || ip.To4() != nil {
				return nil, fmt.Errorf("record %q: invalid IPv6 address %q", name, entry.AAAA)
			}
			addresses.AAAA = ip.To16()
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			return nil, fmt.Errorf("record name must not be empty")
		}
		if !strings.HasSuffix(name, ".") {
			name += "."
		}
		if _, exists := loaded[name]; exists {
			return nil, fmt.Errorf("duplicate record %q", name)
		}
		loaded[name] = addresses
	}
	return loaded, nil
}

func handleQuery(packet []byte) ([]byte, error) {
	request, err := ParseMessage(packet)
	if err != nil {
		return nil, err
	}
	if request.Header.Flags.QR || request.Header.Flags.Opcode != 0 || len(request.Questions) != 1 {
		return nil, fmt.Errorf("unsupported query")
	}

	question := request.Questions[0]
	//initialize the response
	response := Message{
		Header: Header{
			ID: request.Header.ID,
			Flags: Flags{
				QR: true,
				AA: true,
				RD: request.Header.Flags.RD,
			},
		},
		Questions: request.Questions,
	}

	addresses, exists := records[strings.ToLower(question.Name)]
	if !exists {
		response.Header.Flags.RCode = 3 // NXDOMAIN
	}

	var ip net.IP
	if question.Class == 1 { // IN
		switch question.Type {
		case 1: // A
			ip = addresses.A
		case 28: // AAAA
			ip = addresses.AAAA
		}
	}
	if ip != nil {
		response.Answers = []ResourceRecord{{
			Name: question.Name, Type: question.Type, Class: 1,
			TTL: 60, Data: []byte(ip),
		}}
	}

	log.Printf("query: %s type=%d class=%d", question.Name, question.Type, question.Class)
	return response.Marshal()
}
