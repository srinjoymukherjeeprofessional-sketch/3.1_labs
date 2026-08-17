#include "error_injection.h"
#include <stdlib.h>
#include <time.h>

static int recorded_error_position = -1;
static int recorded_error_length = 0;

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
