package main

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// Server is used by one sequential receive loop. Its maps need no mutex.
type Server struct {
	records  map[string]addressRecords
	upstream string
	delay    time.Duration
	timeout  time.Duration
	cache    map[string]cacheEntry // nil for servers started without -main
}

func (s *Server) serve(conn *net.UDPConn) error {
	buf := make([]byte, 65535)
	for {
		n, client, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		// No goroutine: finish this request before reading the next packet.
		response, err := s.handleQuery(buf[:n])
		if err != nil {
			log.Printf("invalid query from %s: %v", client, err)
			continue
		}
		if _, err := conn.WriteToUDP(response, client); err != nil {
			log.Printf("write to %s: %v", client, err)
		}
	}
}

func (s *Server) handleQuery(packet []byte) ([]byte, error) {
	request, err := ParseMessage(packet)
	if err != nil {
		return nil, err
	}
	if request.Header.Flags.QR || request.Header.Flags.Opcode != 0 || len(request.Questions) != 1 {
		return nil, fmt.Errorf("expected a standard query with one question")
	}
	q := request.Questions[0]
	if _, exists := s.records[strings.ToLower(q.Name)]; exists {
		log.Printf("local answer: %s type=%d", q.Name, q.Type)
		return localAnswer(request, s.records, s.upstream != "")
	}
	key, cacheable := queryCacheKey(request)
	if s.cache != nil && cacheable {
		if response, ok := s.cached(key, request, time.Now()); ok {
			log.Printf("cache hit: %s type=%d", q.Name, q.Type)
			return response, nil
		}
	}
	if !request.Header.Flags.RD || s.upstream == "" {
		return s.errorResponse(request, 5) // REFUSED
	}
	log.Printf("forward: %s type=%d upstream=%s delay=%s", q.Name, q.Type, s.upstream, s.delay)
	response, err := forwardQuery(packet, request, s.upstream, s.delay, s.timeout)
	if err != nil {
		log.Printf("upstream failed: %v", err)
		return s.errorResponse(request, 2) // SERVFAIL; never bypass this upstream.
	}
	// Patch fixed header bytes only. RDATA may contain compression pointers.
	setForwardedHeader(response, request)
	if s.cache != nil && cacheable {
		s.store(key, response, time.Now())
	}
	return response, nil
}

func (s *Server) errorResponse(request *Message, code uint8) ([]byte, error) {
	return (&Message{
		Header: Header{ID: request.Header.ID, Flags: Flags{
			QR: true, RD: request.Header.Flags.RD, RA: s.upstream != "", RCode: code,
		}},
		Questions: request.Questions,
	}).Marshal()
}

func setForwardedHeader(packet []byte, request *Message) {
	binary.BigEndian.PutUint16(packet[:2], request.Header.ID)
	flags := binary.BigEndian.Uint16(packet[2:4])
	flags &^= 1<<10 | 1<<8 // Clear AA and restore this client's RD.
	flags |= 1<<15 | 1<<7  // QR, RA
	if request.Header.Flags.RD {
		flags |= 1 << 8
	}
	binary.BigEndian.PutUint16(packet[2:4], flags)
}

func forwardQuery(packet []byte, request *Message, upstream string, delay, timeout time.Duration) ([]byte, error) {
	// Deliberately block this process before sending each upstream query.
	time.Sleep(delay)
	conn, err := net.DialTimeout("udp", upstream, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	query := append([]byte(nil), packet...)
	var id [2]byte
	if _, err := rand.Read(id[:]); err != nil {
		return nil, err
	}
	copy(query[:2], id[:])
	if _, err := conn.Write(query); err != nil {
		return nil, err
	}
	buf := make([]byte, 65535)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		response, err := ParseMessage(buf[:n])
		if err != nil || !response.Header.Flags.QR || response.Header.ID != binary.BigEndian.Uint16(id[:]) || response.Header.Flags.Opcode != request.Header.Flags.Opcode || len(response.Questions) != 1 {
			continue // Ignore unrelated or malformed responses until the deadline.
		}
		got, want := response.Questions[0], request.Questions[0]
		if !strings.EqualFold(got.Name, want.Name) || got.Type != want.Type || got.Class != want.Class {
			continue
		}
		return append([]byte(nil), buf[:n]...), nil
	}
}
