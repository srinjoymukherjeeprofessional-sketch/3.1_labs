// Package utils — TCP connection helpers.
//
// With fixed 64-byte payload, frame sizes on the wire are known and constant:
//   - Sender -> Receiver: DataFrameSize = 15 (header) + 64 (payload) + 4 (trailer) = 83 bytes
//     (Data frames and EOF sentinel are both formatted to 83 bytes on the wire)
//   - Receiver -> Sender: ControlFrameSize = 15 (header) + 0 (payload) + 4 (trailer) = 19 bytes
//     (ACK and NAK frames are both 19 bytes on the wire)
//
// Therefore, no extra length-prefix header is sent on the TCP stream.
// Wire bytes match Fig. 1 directly. io.ReadFull ensures complete frame assembly
// over TCP's byte stream.
package utils

import (
	"io"
	"net"
)

// WriteFrame writes raw frame bytes directly to conn without any extra prefix.
func WriteFrame(conn net.Conn, data []byte) error {
	_, err := conn.Write(data)
	return err
}

// ReadDataFrame reads exactly one Data or EOF frame (83 bytes) from conn.
// It blocks until all 83 bytes arrive using io.ReadFull.
func ReadDataFrame(conn net.Conn) ([]byte, error) {
	buf := make([]byte, DataFrameSize)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadACKFrame reads exactly one ACK or NAK control frame (19 bytes) from conn.
// It blocks until all 19 bytes arrive using io.ReadFull.
func ReadACKFrame(conn net.Conn) ([]byte, error) {
	buf := make([]byte, ControlFrameSize)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}
