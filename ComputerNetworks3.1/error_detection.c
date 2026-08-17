#include "error_detection.h"

// Standard one's-complement checksum over 16-bit words.
unsigned short checksum(const char *data, int len) {
    unsigned int sum = 0;
    int pos = 0;

    while (pos + 1 < len) {
        unsigned int word = ((unsigned char)data[pos] << 8) |
                            (unsigned char)data[pos + 1];
        sum += word;
        sum = (sum & 0xffffU) + (sum >> 16);
        pos += 2;
    }

    if (pos < len) {
        sum += (unsigned char)data[pos] << 8;
        sum = (sum & 0xffffU) + (sum >> 16);
    }

    while (sum >> 16) {
        sum = (sum & 0xffffU) + (sum >> 16);
    }

    return (unsigned short)(~sum & 0xffffU);
}

// CRC-16/CCITT-FALSE:
// polynomial 0x1021, initial value 0xffff, no reflection, xorout 0x0000
unsigned short crc16(const char *data, int len) {
    unsigned short crc = 0xffffU;
    int i;
    int bit;

    for (i = 0; i < len; i++) {
        crc ^= (unsigned short)((unsigned char)data[i] << 8);

        for (bit = 0; bit < 8; bit++) {
            if (crc & 0x8000U) {
                crc = (unsigned short)((crc << 1) ^ 0x1021U);
            } else {
                crc = (unsigned short)(crc << 1);
            }
        }
    }

    return crc;
}

// CRC-10/ATM: polynomial 0x233, initial value 0, no reflection, xorout 0.
unsigned short crc10(const char *data, int len) {
    unsigned short crc = 0;
    int i;
    int bit;

    for (i = 0; i < len; i++) {
        crc ^= (unsigned short)((unsigned char)data[i] << 2);
        for (bit = 0; bit < 8; bit++) {
            if (crc & 0x200U) {
                crc = (unsigned short)(((crc << 1) ^ 0x233U) & 0x3ffU);
            } else {
                crc = (unsigned short)((crc << 1) & 0x3ffU);
            }
        }
    }
    return (unsigned short)(crc & 0x3ffU);
}

// CRC-8/ATM: polynomial 0x07, initial value 0, no reflection, xorout 0.
unsigned char crc8(const char *data, int len) {
    unsigned char crc = 0;
    int i;
    int bit;

    for (i = 0; i < len; i++) {
        crc ^= (unsigned char)data[i];
        for (bit = 0; bit < 8; bit++) {
            if (crc & 0x80U) {
                crc = (unsigned char)((crc << 1) ^ 0x07U);
            } else {
                crc = (unsigned char)(crc << 1);
            }
        }
    }
    return crc;
}

// CRC-32/IEEE:
// polynomial 0xedb88320 in reflected form, initial value 0xffffffff,
// reflected input/output, xorout 0xffffffff
unsigned int crc32(const char *data, int len) {
    unsigned int crc = 0xffffffffU;
    int i;
    int bit;

    for (i = 0; i < len; i++) {
        crc ^= (unsigned int)(unsigned char)data[i];

        for (bit = 0; bit < 8; bit++) {
            if (crc & 1U) {
                crc = (crc >> 1) ^ 0xedb88320U;
            } else {
                crc >>= 1;
            }
        }
    }

    return crc ^ 0xffffffffU;
}
