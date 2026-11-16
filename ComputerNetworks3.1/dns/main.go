package main

import (
	"encoding/binary"
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

func handleQuery(query []byte) ([]byte, error) {
	const headerSize = 12

	if len(query) < headerSize {
		return nil, fmt.Errorf("packet shorter than DNS header")
	}

	flags := binary.BigEndian.Uint16(query[2:4])
	questions := binary.BigEndian.Uint16(query[4:6])

	// Handle only standard queries with exactly one question.
	if flags&0x8000 != 0 || flags&0x7800 != 0 || questions != 1 {
		return nil, fmt.Errorf("unsupported query")
	}

	name, offset, err := readName(query, headerSize)
	if err != nil {
		return nil, err
	}
	if offset+4 > len(query) {
		return nil, fmt.Errorf("incomplete question")
	}

	qtype := binary.BigEndian.Uint16(query[offset : offset+2])
	qclass := binary.BigEndian.Uint16(query[offset+2 : offset+4])
	questionEnd := offset + 4

	// Start a new response with the original header and question.
	response := make([]byte, questionEnd)
	copy(response, query[:questionEnd])

	// QR = response, AA = authoritative; preserve the request's RD bit.
	responseFlags := uint16(0x8400) | (flags & 0x0100)
	addresses, exists := records[strings.ToLower(name)]

	if !exists {
		responseFlags |= 3 // RCODE = NXDOMAIN
	}

	binary.BigEndian.PutUint16(response[2:4], responseFlags)
	binary.BigEndian.PutUint16(response[4:6], 1) // QDCOUNT
	binary.BigEndian.PutUint16(response[6:8], 0) // ANCOUNT
	binary.BigEndian.PutUint16(response[8:10], 0)
	binary.BigEndian.PutUint16(response[10:12], 0)

	var ip net.IP
	if qclass == 1 { // IN
		switch qtype {
		case 1: // A: 4 bytes
			ip = addresses.A
		case 28: // AAAA: 16 bytes
			ip = addresses.AAAA
		}
	}

	// An existing name without the requested type gets an empty answer.
	if ip != nil {
		binary.BigEndian.PutUint16(response[6:8], 1)

		// NAME points back to the question name at offset 12.
		response = append(response,
			0xc0, 0x0c, // compressed NAME
			byte(qtype>>8), byte(qtype), // TYPE: A or AAAA
			0x00, 0x01, // CLASS: IN
			0x00, 0x00, 0x00, 0x3c, // TTL: 60 seconds
			0x00, byte(len(ip)), // RDLENGTH: 4 or 16 bytes
		)
		response = append(response, ip...)
	}

	log.Printf("query: %s type=%d class=%d", name, qtype, qclass)
	return response, nil
}

// readName decodes an uncompressed DNS name from the question.
func readName(packet []byte, offset int) (string, int, error) {
	var labels []string
	start := offset

	for {
		if offset >= len(packet) {
			return "", 0, fmt.Errorf("incomplete name")
		}

		length := int(packet[offset])
		offset++

		if length == 0 {
			if offset-start > 255 {
				return "", 0, fmt.Errorf("name too long")
			}
			if len(labels) == 0 {
				return ".", offset, nil
			}
			return strings.Join(labels, ".") + ".", offset, nil
		}

		if length > 63 || offset+length > len(packet) {
			return "", 0, fmt.Errorf("invalid label")
		}

		labels = append(labels, string(packet[offset:offset+length]))
		offset += length
	}
}
