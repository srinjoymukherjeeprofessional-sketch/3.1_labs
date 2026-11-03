#include "error_injection.h"
#include <stdlib.h>
#include <time.h>

static int recorded_error_position = -1;
static int recorded_error_length = 0;

static int hex_value(char c) {
    if (c >= '0' && c <= '9') {
        return c - '0';
    }
    if (c >= 'a' && c <= 'f') {
        return c - 'a' + 10;
    }
    if (c >= 'A' && c <= 'F') {
        return c - 'A' + 10;
    }
    return -1;
}

int last_error_position(void) {
    return recorded_error_position;
}

int last_error_length(void) {
    return recorded_error_length;
}

void seed_error_injection(void) {
    srand((unsigned int)time(NULL));
}

void seed_error_injection_value(unsigned int seed) {
    srand(seed);
}

int inject_at_position(char *packet, int packet_size, int position) {
    // Returns 0 if successful, 1 if the position is invalid.
    if (position < 0 || position >= packet_size) {
        return 1;
    }

    packet[position] ^= 1;
    recorded_error_position = position;
    recorded_error_length = 1;
    return 0;
}

int inject_random_position(char *packet, int packet_size) {
    int position;

    if (packet == NULL || packet_size <= 0) {
        return 1;
    }

    position = rand() % packet_size;
    return inject_at_position(packet, packet_size, position);
}

int inject_burst(char *packet, int packet_size, int burst_length) {
    int start_position;
    int i;

    if (packet == NULL || packet_size <= 0 || burst_length <= 0 ||
        burst_length > packet_size) {
        return 1;
    }

    start_position = rand() % (packet_size - burst_length + 1);

    for (i = 0; i < burst_length; i++) {
        packet[start_position + i] ^= 1;
    }
    recorded_error_position = start_position;
    recorded_error_length = burst_length;

    return 0;
}

int inject_xor_mask_hex(char *packet, int packet_size, const char *mask_hex) {
    int byte_index = 0;
    int first_changed = -1;
    int last_changed = -1;

    if (packet == NULL || mask_hex == NULL || packet_size <= 0) {
        return 1;
    }

    while (mask_hex[0] != '\0') {
        int high;
        int low;
        unsigned char mask;

        if (mask_hex[1] == '\0' || byte_index >= packet_size) {
            return 1;
        }

        high = hex_value(mask_hex[0]);
        low = hex_value(mask_hex[1]);
        if (high < 0 || low < 0) {
            return 1;
        }

        mask = (unsigned char)((high << 4) | low);
        if (mask != 0) {
            packet[byte_index] ^= (char)mask;
            if (first_changed < 0) {
                first_changed = byte_index;
            }
            last_changed = byte_index;
        }

        byte_index++;
        mask_hex += 2;
    }

    recorded_error_position = first_changed;
    recorded_error_length =
        first_changed < 0 ? 0 : last_changed - first_changed + 1;
    return 0;
}
