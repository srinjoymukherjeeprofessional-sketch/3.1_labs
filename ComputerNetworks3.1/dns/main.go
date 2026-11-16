package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

// This server is authoritative for these names.
type addressRecords struct {
	A    net.IP
	AAAA net.IP
	PTR  string
}

func main() {
	isMain := flag.Bool("main", false, "enable the client-facing cache; upstream must be loopback")
	port := flag.Int("port", 5353, "UDP port on 127.0.0.1")
	path := flag.String("records", "records.json", "records JSON file")
	upstream := flag.String("upstream", "", "upstream IP:port (default: next loopback port)")
	delay := flag.Duration("delay", time.Second, "delay before each upstream query")
	timeout := flag.Duration("timeout", 10*time.Second, "upstream response timeout, excluding artificial delay")
	flag.Parse()
	if *port < 1 || *port > 65535 || *delay < 0 || *timeout <= 0 {
		log.Fatal("invalid port, delay, or timeout")
	}
	if *upstream == "" {
		if *port == 65535 {
			log.Fatal("specify -upstream when using port 65535")
		}
		*upstream = fmt.Sprintf("127.0.0.1:%d", *port+1)
	}
	upstreamAddr, err := net.ResolveUDPAddr("udp", *upstream)
	if err != nil || upstreamAddr.IP == nil || upstreamAddr.Port < 1 || upstreamAddr.Port > 65535 {
		log.Fatal("upstream must be a valid IP:port")
	}
	if *isMain && !upstreamAddr.IP.IsLoopback() {
		log.Fatal("-main requires a loopback upstream; it cannot query public resolvers directly")
	}
	if upstreamAddr.IP.IsLoopback() && upstreamAddr.Port == *port {
		log.Fatal("upstream cannot be this server")
	}
	loaded, err := loadRecords(*path)
	if err != nil {
		log.Fatal(err)
	}
	server := &Server{records: loaded, upstream: upstreamAddr.String(), delay: *delay, timeout: *timeout}
	if *isMain {
		server.cache = make(map[string]cacheEntry)
	}
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: *port}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	log.Printf("DNS listening=%s records=%s upstream=%s delay=%s cache=%t", addr, *path, server.upstream, server.delay, *isMain)
	if err := server.serve(conn); err != nil {
		log.Fatal(err)
	}
}

// loadRecords reads each domain's A, AAAA, and PTR records once at startup.
func loadRecords(path string) (map[string]addressRecords, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read records: %w", err)
	}

	var entries map[string]struct {
		A    string `json:"A"`
		AAAA string `json:"AAAA"`
		PTR  string `json:"PTR"`
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
		if entry.PTR != "" {
			target := strings.TrimSpace(entry.PTR)
			// Validate DNS label lengths before accepting a PTR target.
			if _, err := appendName(nil, target, make(map[string]uint16)); err != nil {
				return nil, fmt.Errorf("record %q: invalid PTR target %q: %w", name, entry.PTR, err)
			}
			addresses.PTR = strings.TrimSuffix(target, ".") + "."
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

func localAnswer(request *Message, localRecords map[string]addressRecords, recursion bool) ([]byte, error) {
	question := request.Questions[0]
	var err error
	// Initialize the response
	response := Message{
		Header: Header{
			ID: request.Header.ID,
			Flags: Flags{
				QR: true,
				AA: true,
				RD: request.Header.Flags.RD,
				RA: recursion,
			},
		},
		Questions: request.Questions,
	}

	addresses, exists := localRecords[strings.ToLower(question.Name)]
	if !exists {
		response.Header.Flags.RCode = 3 // NXDOMAIN
	}

	var data []byte
	if question.Class == 1 { // IN
		switch question.Type {
		case 1: // A
			data = addresses.A
		case 28: // AAAA
			data = addresses.AAAA
		case 12: // PTR
			data, err = handlePTR(addresses.PTR)
			if err != nil {
				return nil, err
			}
		}
	}
	if data != nil {
		response.Answers = []ResourceRecord{{
			Name: question.Name, Type: question.Type, Class: 1,
			TTL: 60, Data: data,
		}}
	}

	log.Printf("query: %s type=%d class=%d", question.Name, question.Type, question.Class)
	return response.Marshal()
}
