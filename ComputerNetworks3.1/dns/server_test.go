package main

import (
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"
)

// Test helpers run separate sequential servers concurrently, like separate OS
// processes. The production server never launches a request goroutine.
func startTestServer(t *testing.T, server *Server) string {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.serve(conn) }()
	t.Cleanup(func() {
		conn.Close()
		if err := <-done; !errors.Is(err, net.ErrClosed) {
			t.Errorf("server stopped: %v", err)
		}
	})
	return conn.LocalAddr().String()
}

func testQuery(t *testing.T, name string, rd bool, id uint16) []byte {
	t.Helper()
	packet, err := (&Message{
		Header:    Header{ID: id, Flags: Flags{RD: rd}},
		Questions: []Question{{Name: name, Type: 1, Class: 1}},
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

func testExchange(t *testing.T, addr string, query []byte) *Message {
	t.Helper()
	conn, err := net.Dial("udp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(query); err != nil {
		t.Fatal(err)
	}
	packet := make([]byte, 65535)
	n, err := conn.Read(packet)
	if err != nil {
		t.Fatal(err)
	}
	message, err := ParseMessage(packet[:n])
	if err != nil {
		t.Fatal(err)
	}
	return message
}

func TestSequentialForwardingChain(t *testing.T) {
	// A local fixture stands in for the public resolver.
	public := startTestServer(t, &Server{records: map[string]addressRecords{
		"public.test.": {A: net.ParseIP("192.0.2.40").To4()},
	}})
	third := startTestServer(t, &Server{
		records:  map[string]addressRecords{"third.test.": {A: net.ParseIP("192.0.2.30").To4()}},
		upstream: public, timeout: time.Second,
	})
	second := startTestServer(t, &Server{
		records:  map[string]addressRecords{"second.test.": {A: net.ParseIP("192.0.2.20").To4()}},
		upstream: third, timeout: time.Second,
	})
	main := &Server{
		records:  map[string]addressRecords{"main.test.": {A: net.ParseIP("192.0.2.10").To4()}},
		upstream: second, timeout: time.Second, cache: make(map[string]cacheEntry),
	}
	addr := startTestServer(t, main)
	for _, tc := range []struct{ name, ip string }{
		{"main.test.", "192.0.2.10"}, {"second.test.", "192.0.2.20"},
		{"third.test.", "192.0.2.30"}, {"public.test.", "192.0.2.40"},
	} {
		response := testExchange(t, addr, testQuery(t, tc.name, true, 123))
		if response.Header.ID != 123 || !response.Header.Flags.RA || response.Header.Flags.AA != (tc.name == "main.test.") || len(response.Answers) != 1 || net.IP(response.Answers[0].Data).String() != tc.ip {
			t.Fatalf("unexpected response for %s: %+v", tc.name, response)
		}
	}
	// Changing transaction IDs must not prevent a cache hit. Stop relying on
	// the upstream by checking the cache after the serving loop has stopped.
	response := testExchange(t, addr, testQuery(t, "PUBLIC.test.", true, 456))
	if response.Header.ID != 456 || len(response.Answers) != 1 {
		t.Fatalf("unexpected cached answer: %+v", response)
	}
	refused := testExchange(t, addr, testQuery(t, "unknown.test.", false, 789))
	if refused.Header.Flags.RCode != 5 {
		t.Fatalf("expected REFUSED, got %+v", refused.Header)
	}
	// A known local name without this type must not be forwarded.
	request, _ := ParseMessage(testQuery(t, "main.test.", true, 321))
	request.Questions[0].Type = 28
	packet, _ := request.Marshal()
	empty := testExchange(t, addr, packet)
	if empty.Header.Flags.RCode != 0 || len(empty.Answers) != 0 || !empty.Header.Flags.AA {
		t.Fatalf("expected local empty answer, got %+v", empty)
	}
}

func TestForwardingDelayAndCacheHit(t *testing.T) {
	upstream := startTestServer(t, &Server{records: map[string]addressRecords{
		"delay.test.": {A: net.ParseIP("192.0.2.1").To4()},
	}})
	s := &Server{upstream: upstream, delay: 50 * time.Millisecond, timeout: time.Second, cache: make(map[string]cacheEntry)}
	start := time.Now()
	if _, err := s.handleQuery(testQuery(t, "delay.test.", true, 1)); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) < s.delay {
		t.Fatal("forwarding did not apply its delay")
	}
	// An unusable upstream proves that the second query really uses the cache.
	s.upstream = "127.0.0.1:0"
	packet, err := s.handleQuery(testQuery(t, "DELAY.test.", true, 2))
	if err != nil {
		t.Fatal(err)
	}
	response, _ := ParseMessage(packet)
	if response.Header.ID != 2 || response.Header.Flags.RCode != 0 || len(response.Answers) != 1 {
		t.Fatalf("cache did not answer: %+v", response)
	}
	// Local records take precedence even when an old forwarded entry exists.
	s.records = map[string]addressRecords{"delay.test.": {A: net.ParseIP("192.0.2.99").To4()}}
	packet, _ = s.handleQuery(testQuery(t, "delay.test.", true, 3))
	response, _ = ParseMessage(packet)
	if !response.Header.Flags.AA || net.IP(response.Answers[0].Data).String() != "192.0.2.99" {
		t.Fatal("local record did not take precedence")
	}
}

func TestUpstreamFailureAndMismatchedResponses(t *testing.T) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		n, peer, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		request, _ := ParseMessage(buf[:n])
		// Wrong transaction ID, then wrong question: neither should be accepted.
		response := Message{Header: Header{ID: request.Header.ID + 1, Flags: Flags{QR: true}}, Questions: request.Questions}
		packet, _ := response.Marshal()
		conn.WriteToUDP(packet, peer)
		response.Header.ID = request.Header.ID
		response.Questions = []Question{{Name: "wrong.test.", Type: 1, Class: 1}}
		packet, _ = response.Marshal()
		conn.WriteToUDP(packet, peer)
	}()
	s := &Server{upstream: conn.LocalAddr().String(), timeout: 50 * time.Millisecond}
	packet, err := s.handleQuery(testQuery(t, "unknown.test.", true, 42))
	if err != nil {
		t.Fatal(err)
	}
	<-done
	response, _ := ParseMessage(packet)
	if response.Header.ID != 42 || response.Header.Flags.RCode != 2 || response.Header.Flags.AA {
		t.Fatalf("expected SERVFAIL: %+v", response)
	}
}

func TestCacheTTLsAndCompression(t *testing.T) {
	request, _ := ParseMessage(testQuery(t, "public.test.", true, 1))
	// CNAME's target uses a pointer to 'test' at offset 19 in the question.
	response := Message{
		Header: Header{ID: 1, Flags: Flags{QR: true, RA: true}}, Questions: request.Questions,
		Answers: []ResourceRecord{
			{Name: "public.test.", Type: 5, Class: 1, TTL: 60, Data: []byte{6, 't', 'a', 'r', 'g', 'e', 't', 0xc0, 19}},
			{Name: "target.test.", Type: 1, Class: 1, TTL: 30, Data: []byte{192, 0, 2, 1}},
		},
		Additional: []ResourceRecord{{Name: ".", Type: 41, Class: 1232, TTL: 0x8000}},
	}
	packet, err := response.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cache: make(map[string]cacheEntry)}
	key, _ := queryCacheKey(request)
	now := time.Now()
	s.store(key, packet, now)
	request.Header.ID = 999
	cached, ok := s.cached(key, request, now.Add(7*time.Second))
	if !ok {
		t.Fatal("cache miss")
	}
	message, err := ParseMessage(cached)
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.ID != 999 || message.Answers[0].TTL != 53 || message.Answers[1].TTL != 23 || message.Additional[0].TTL != 0x8000 {
		t.Fatalf("incorrect cached TTL/header: %+v", message)
	}
	_, nameStart, _ := readName(cached, 12)
	_, recordStart, _ := readName(cached, nameStart+4)
	target, _, err := readName(cached, recordStart+10)
	if err != nil || target != "target.test." {
		t.Fatalf("compression broken: %q %v", target, err)
	}
	if _, ok := s.cached(key, request, now.Add(30*time.Second)); ok || len(s.cache) != 0 {
		t.Fatal("cache entry did not expire at its shortest TTL")
	}
	// TC, NXDOMAIN, and zero-TTL responses aren't cached.
	for _, mutate := range []func([]byte){
		func(p []byte) { p[2] |= 2 },
		func(p []byte) { p[3] |= 3 },
		func(p []byte) { binary.BigEndian.PutUint32(p[recordStart+4:recordStart+8], 0) },
	} {
		copyPacket := append([]byte(nil), packet...)
		mutate(copyPacket)
		s.store(key, copyPacket, now)
		if len(s.cache) != 0 {
			t.Fatal("cached an ineligible response")
		}
	}
}
