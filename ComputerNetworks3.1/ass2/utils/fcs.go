package utils

import "encoding/binary"

// Checksum calculates the 16-bit Internet Checksum (RFC 1071 / 1's complement sum) over data.
func Checksum(data []byte) uint16 {
	var sum uint32 = 0
	for i := 0; i <= len(data)-2; i += 2 {
		chunk := binary.LittleEndian.Uint16(data[i : i+2])
		sum += uint32(chunk)
	}

	if len(data)%2 != 0 {
		sum += uint32(data[len(data)-1])
	}

	for (sum >> 16) > 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}

	return uint16(^sum)
}

// CRC calculates a generic CRC value for a given generator polynomial and bit-width.
func CRC(data []byte, polynomial uint64, width uint) uint64 {
	var crc uint64 = 0
	var top_bit uint64 = uint64(1) << (width - 1)
	var mask uint64

	if width == 64 {
		mask = 0xFFFFFFFFFFFFFFFF
	} else {
		mask = (uint64(1) << width) - 1
	}

	for i := range data {
		crc ^= uint64(data[i]) << (width - 8)
		for j := 0; j < 8; j++ {
			if crc&top_bit != 0 {
				crc = (crc << 1) ^ polynomial
			} else {
				crc <<= 1
			}
			crc &= mask
		}
	}
	return crc
}

// CRC8 computes 8-bit CRC using polynomial 0xD5
func CRC8(data []byte) uint8 {
	return uint8(CRC(data, 0xD5, 8))
}

// CRC10 computes 10-bit CRC using polynomial 0x289
func CRC10(data []byte) uint16 {
	return uint16(CRC(data, 0x289, 10))
}

// CRC16 computes 16-bit CRC using polynomial 0x8005 (CRC-16-IBM)
func CRC16(data []byte) uint16 {
	return uint16(CRC(data, 0x8005, 16))
}

// CRC32 computes 32-bit CRC using polynomial 0x04C11DB7 (IEEE 802.3 Ethernet)
func CRC32(data []byte) uint32 {
	return uint32(CRC(data, 0x04C11DB7, 32))
}
