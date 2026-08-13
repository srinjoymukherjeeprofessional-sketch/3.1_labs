/*
 * Standalone evaluation driver.
 *
 * Build:
 *   gcc -O2 -Wall -Wextra -pedantic evaluation.c error_detection.c \
 *       error_injection.c -o evaluation
 *
 * Output:
 *   evaluation_results.csv
 *
 * The existing sender/receiver files are not modified or required at runtime.
 */

#define _POSIX_C_SOURCE 200809L

#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <time.h>

#include "error_detection.h"
#include "error_injection.h"

#define PACKET_SIZE 64
#define PROTECTED_SIZE 60
#define SINGLE_TRIALS PROTECTED_SIZE
#define RANDOM_ERROR_TRIALS 10000
#define TIMING_ITERATIONS 100000

static volatile uint32_t timing_sink;

static uint64_t now_ns(void)
{
    struct timespec t;
    clock_gettime(CLOCK_MONOTONIC, &t);
    return (uint64_t)t.tv_sec * 1000000000ULL + (uint64_t)t.tv_nsec;
}

static uint32_t read_be(const unsigned char *p, size_t n)
{
    uint32_t value = 0;
    size_t i;
    for (i = 0; i < n; i++) {
        value = (value << 8) | p[i];
    }
    return value;
}

static void write_be(unsigned char *p, size_t n, uint32_t value)
{
    while (n > 0) {
        p[--n] = (unsigned char)(value & 0xffU);
        value >>= 8;
    }
}

static void make_packet(unsigned char packet[PACKET_SIZE])
{
    size_t i;
    for (i = 0; i < PROTECTED_SIZE; i++) {
        packet[i] = (unsigned char)((i * 37U + 11U) & 0xffU);
    }
    memset(packet + PROTECTED_SIZE, 0, PACKET_SIZE - PROTECTED_SIZE);
}

static void encode(unsigned char packet[PACKET_SIZE], int scheme)
{
    uint32_t value;

    if (scheme == 0) {
        value = checksum((const char *)packet, PROTECTED_SIZE);
        write_be(packet + 60, 2, value);
    } else if (scheme == 1) {
        value = crc16((const char *)packet, PROTECTED_SIZE);
        write_be(packet + 60, 2, value);
    } else {
        value = crc32((const char *)packet, PROTECTED_SIZE);
        write_be(packet + 60, 4, value);
    }
}

static int detected(const unsigned char packet[PACKET_SIZE], int scheme)
{
    uint32_t expected;
    uint32_t received;

    if (scheme == 0) {
        expected = checksum((const char *)packet, PROTECTED_SIZE);
        received = read_be(packet + 60, 2);
    } else if (scheme == 1) {
        expected = crc16((const char *)packet, PROTECTED_SIZE);
        received = read_be(packet + 60, 2);
    } else {
        expected = crc32((const char *)packet, PROTECTED_SIZE);
        received = read_be(packet + 60, 4);
    }
    return expected != received;
}

static const char *comparison(int a, int b)
{
    if (a && b) return "both_detected";
    if (a && !b) return "checksum_only";
    if (!a && b) return "crc_only";
    return "neither_detected";
}

static void csv_detail(FILE *out, const char *error_type, int trial,
                       int position, int burst_length,
                       const unsigned char original[PACKET_SIZE],
                       const unsigned char corrupted[PACKET_SIZE])
{
    unsigned char packet[PACKET_SIZE];
    int c;
    int c16;
    int c32;

    /* Give each scheme the same corrupted protected bytes, but its own
     * correctly generated integrity field. */
    memcpy(packet, original, sizeof packet);
    encode(packet, 0);
    memcpy(packet, corrupted, PROTECTED_SIZE);
    c = detected(packet, 0);
    memcpy(packet, original, sizeof packet);
    encode(packet, 1);
    memcpy(packet, corrupted, PROTECTED_SIZE);
    c16 = detected(packet, 1);
    memcpy(packet, original, sizeof packet);
    encode(packet, 2);
    memcpy(packet, corrupted, PROTECTED_SIZE);
    c32 = detected(packet, 2);

    fprintf(out,
            "detail,%s,%d,%d,%d,%d,%d,%d,%s,%s,,,,,,\n",
            error_type, trial, position, burst_length, c, c16, c32,
            comparison(c, c16), comparison(c, c32));
}

static double elapsed_validation_ns(const unsigned char packet[PACKET_SIZE],
                                    int scheme)
{
    uint64_t start = now_ns();
    int i;

    for (i = 0; i < TIMING_ITERATIONS; i++) {
        timing_sink ^= (uint32_t)detected(packet, scheme);
    }
    return (double)(now_ns() - start) / (double)TIMING_ITERATIONS;
}

static void benchmark(FILE *out, const unsigned char packet[PACKET_SIZE],
                      int scheme, const char *name, const char *polynomial)
{
    struct rusage before;
    struct rusage after;
    double average_ns;

    getrusage(RUSAGE_SELF, &before);
    average_ns = elapsed_validation_ns(packet, scheme);
    getrusage(RUSAGE_SELF, &after);

    fprintf(out,
            "benchmark,clean_packet,%d,-1,0,,,,,,%s,%s,%.2f,%.3f,%.3f,%ld\n",
            TIMING_ITERATIONS, name, polynomial, average_ns,
            (after.ru_utime.tv_sec - before.ru_utime.tv_sec) * 1000.0 +
                (after.ru_utime.tv_usec - before.ru_utime.tv_usec) / 1000.0,
            (after.ru_stime.tv_sec - before.ru_stime.tv_sec) * 1000.0 +
                (after.ru_stime.tv_usec - before.ru_stime.tv_usec) / 1000.0,
            after.ru_maxrss);
}

int main(void)
{
    FILE *out;
    unsigned char original[PACKET_SIZE];
    unsigned char packet[PACKET_SIZE];
    int position;
    int burst_length;
    int trial = 0;
    const int burst_lengths[] = {1, 2, 4, 8, 16, 32};
    size_t b;

    out = fopen("evaluation_results.csv", "w");
    if (out == NULL) {
        perror("evaluation_results.csv");
        return EXIT_FAILURE;
    }

    fprintf(out,
            "row_type,error_type,trial,position,burst_length,checksum_detected,"
            "crc16_detected,crc32_detected,checksum_vs_crc16,checksum_vs_crc32,"
            "scheme,polynomial,avg_validation_ns,user_cpu_ms,system_cpu_ms,max_rss_kb\n");

    make_packet(original);

    /* Exhaustive one-bit errors over every protected byte. */
    for (position = 0; position < PROTECTED_SIZE; position++) {
        memcpy(packet, original, sizeof packet);
        encode(packet, 0);
        inject_at_position((char *)packet, PROTECTED_SIZE, position);
        csv_detail(out, "single_bit", trial++, position, 1, original, packet);
    }

    /* Randomized burst errors.  Re-seeding before each scheme is unnecessary
     * here because the same corrupted protected bytes are tested by all
     * validators from one packet. */
    seed_error_injection();
    for (b = 0; b < sizeof burst_lengths / sizeof burst_lengths[0]; b++) {
        burst_length = burst_lengths[b];
        for (trial = 0; trial < 100; trial++) {
            memcpy(packet, original, sizeof packet);
            encode(packet, 0);
            inject_burst((char *)packet, PROTECTED_SIZE, burst_length);
            csv_detail(out, "burst", trial, -1, burst_length, original, packet);
        }
    }

    /* Random multi-bit errors are useful for finding the requested cases
     * where one detector misses an error that another detector catches. */
    srand(20260814U);
    for (trial = 0; trial < RANDOM_ERROR_TRIALS; trial++) {
        int flips = 2 + rand() % 7;
        int flip;

        memcpy(packet, original, sizeof packet);
        for (flip = 0; flip < flips; flip++) {
            position = rand() % PROTECTED_SIZE;
            inject_at_position((char *)packet, PROTECTED_SIZE, position);
        }
        csv_detail(out, "random_multi_bit", trial, -1, flips,
                   original, packet);
    }

    /* Random bit positions (rather than only the least-significant bit) give
     * broader coverage of checksum/CRC differences. */
    srand(8142026U);
    for (trial = 0; trial < RANDOM_ERROR_TRIALS; trial++) {
        int flips = 2 + rand() % 15;
        int flip;

        memcpy(packet, original, sizeof packet);
        for (flip = 0; flip < flips; flip++) {
            position = rand() % PROTECTED_SIZE;
            packet[position] ^= (unsigned char)(1U << (rand() % 8));
        }
        csv_detail(out, "random_bit_pattern", trial, -1, flips,
                   original, packet);
    }

    /* Timing and process resource measurements for each scheme. */
    for (position = 0; position < 3; position++) {
        const char *name = position == 0 ? "checksum" :
                           position == 1 ? "crc16" : "crc32";
        const char *polynomial = position == 0 ? "ones_complement" :
                                 position == 1 ? "0x1021" : "0xEDB88320";
        memcpy(packet, original, sizeof packet);
        encode(packet, position);
        benchmark(out, packet, position, name, polynomial);
    }

    fclose(out);
    printf("Evaluation complete. Results written to evaluation_results.csv\n");
    printf("Timing sink: %u\n", timing_sink);
    return EXIT_SUCCESS;
}
