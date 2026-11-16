package main

import (
	"encoding/binary"
	"strings"
	"time"
)

type ttlField struct {
	offset int
	ttl    uint32
}

type cacheEntry struct {
	packet  []byte
	stored  time.Time
	expires time.Time
	ttls    []ttlField
}

// Include the canonical question, flags, and EDNS parameters in the key so
// clients with different DNSSEC or UDP-size settings don't share an answer.
func queryCacheKey(request *Message) (string, bool) {
	if len(request.Questions) != 1 || len(request.Answers) != 0 || len(request.Authority) != 0 {
		return "", false
	}
	for _, rr := range request.Additional {
		if rr.Type != 41 || rr.Name != "." {
			return "", false // Only OPT metadata is supported in cached queries.
		}
	}
	canonical := *request
	canonical.Header.ID = 0
	q := request.Questions[0]
	q.Name = strings.ToLower(q.Name)
	canonical.Questions = []Question{q}
	packet, err := canonical.Marshal()
	return string(packet), err == nil
}

func (s *Server) cached(key string, request *Message, now time.Time) ([]byte, bool) {
	entry, exists := s.cache[key]
	if !exists {
		return nil, false
	}
	if !now.Before(entry.expires) {
		delete(s.cache, key)
		return nil, false
	}
	packet := append([]byte(nil), entry.packet...)
	age := uint32(now.Sub(entry.stored) / time.Second)
	for _, field := range entry.ttls {
		binary.BigEndian.PutUint32(packet[field.offset:field.offset+4], field.ttl-age)
	}
	setForwardedHeader(packet, request)
	return packet, true
}

func (s *Server) store(key string, packet []byte, now time.Time) {
	message, err := ParseMessage(packet)
	if err != nil || message.Header.Flags.RCode != 0 || message.Header.Flags.TC || len(message.Answers) == 0 {
		return // No errors, negative answers, or truncated responses are cached.
	}
	var fields []ttlField
	minimum := ^uint32(0)
	offset := 12
	for range message.Questions {
		_, next, _ := readName(packet, offset)
		offset = next + 4
	}
	for _, section := range [][]ResourceRecord{message.Answers, message.Authority, message.Additional} {
		for _, rr := range section {
			_, next, _ := readName(packet, offset)
			if rr.Type == 41 {
				// OPT's TTL field holds extended RCode/version/flags, not a TTL.
				if rr.TTL>>24 != 0 {
					return
				}
			} else {
				if rr.TTL == 0 || rr.TTL > 0x7fffffff {
					return
				}
				fields = append(fields, ttlField{offset: next + 4, ttl: rr.TTL})
				if rr.TTL < minimum {
					minimum = rr.TTL
				}
			}
			offset = next + 10 + len(rr.Data)
		}
	}
	if len(fields) == 0 {
		return
	}
	// Bound the cache, discarding expired entries first. Everything is sequential.
	if len(s.cache) >= 1024 {
		for k, entry := range s.cache {
			if !now.Before(entry.expires) {
				delete(s.cache, k)
			}
		}
		if len(s.cache) >= 1024 {
			for k := range s.cache {
				delete(s.cache, k)
				break
			}
		}
	}
	s.cache[key] = cacheEntry{
		packet: append([]byte(nil), packet...), stored: now,
		expires: now.Add(time.Duration(minimum) * time.Second), ttls: fields,
	}
}
