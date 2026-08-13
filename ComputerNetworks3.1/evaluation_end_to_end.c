/*
 * End-to-end evaluator. This program runs the actual receiver and sender
 * executables; it does not call checksum/CRC functions directly.
 *
 * Build first:
 *   gcc -O2 sender.c error_detection.c error_injection.c -o sender
 *   gcc -O2 receiver.c error_detection.c -o receiver
 *   gcc -O2 evaluation_end_to_end.c -o evaluation_end_to_end
 *
 * Run from this directory:
 *   ./evaluation_end_to_end
 *
 * Output:
 *   evaluation_end_to_end.csv
 *   evaluation_receiver.log
 */

#define _GNU_SOURCE
#define _POSIX_C_SOURCE 200809L

#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <time.h>
#include <unistd.h>

#define TOTAL_CASES 100
#define SCHEMES 4
#define TOTAL_TRIALS (TOTAL_CASES * SCHEMES)
#define RECEIVER_LOG "evaluation_receiver.log"
#define OUTPUT_CSV "evaluation_end_to_end.csv"
#define PACKET_OUTPUT_CSV "evaluation_packets.csv"

static const char *scheme_name(int checktype)
{
    switch (checktype) {
    case 0: return "checksum";
    case 1: return "crc16";
    case 2: return "crc32";
    default: return "crc10";
    }
}

static const char *mode_name(int mode)
{
    return mode == 0 ? "clean" : mode == 1 ? "single_bit" : "burst";
}

static double milliseconds_since(const struct timespec *start)
{
    struct timespec now;
    clock_gettime(CLOCK_MONOTONIC, &now);
    return (now.tv_sec - start->tv_sec) * 1000.0 +
           (now.tv_nsec - start->tv_nsec) / 1000000.0;
}

static int start_receiver(void)
{
    pid_t pid = fork();
    int log_fd;

    if (pid < 0) {
        return -1;
    }
    if (pid == 0) {
        log_fd = open(RECEIVER_LOG, O_WRONLY | O_CREAT | O_TRUNC, 0600);
        if (log_fd < 0) {
            _exit(127);
        }
        dup2(log_fd, STDOUT_FILENO);
        dup2(log_fd, STDERR_FILENO);
        close(log_fd);
        setenv("EVAL_DETAIL", "1", 1);
        execl("./receiver", "./receiver", (char *)NULL);
        _exit(127);
    }

    /* Give bind/listen time to complete before the first sender. */
    usleep(300000);
    return (int)pid;
}

static int run_sender(int mode, int burst_length, int checktype,
                      unsigned int seed,
                      struct rusage *usage, double *wall_ms)
{
    int input_pipe[2];
    char input[64];
    pid_t pid;
    int status;
    struct timespec start;

    if (pipe(input_pipe) < 0) {
        return -1;
    }

    pid = fork();
    if (pid < 0) {
        close(input_pipe[0]);
        close(input_pipe[1]);
        return -1;
    }
    if (pid == 0) {
        char seed_text[32];

        close(input_pipe[1]);
        dup2(input_pipe[0], STDIN_FILENO);
        close(input_pipe[0]);
        snprintf(seed_text, sizeof seed_text, "%u", seed);
        setenv("ERROR_INJECTION_SEED", seed_text, 1);
        setenv("EVAL_DETAIL", "1", 1);
        execl("./sender", "./sender", "127.0.0.1", (char *)NULL);
        _exit(127);
    }

    close(input_pipe[0]);
    if (mode == 2) {
        snprintf(input, sizeof input, "%d\n%d\n%d\n",
                 mode, burst_length, checktype);
    } else {
        snprintf(input, sizeof input, "%d\n%d\n", mode, checktype);
    }
    (void)write(input_pipe[1], input, strlen(input));
    close(input_pipe[1]);

    clock_gettime(CLOCK_MONOTONIC, &start);
    if (wait4(pid, &status, 0, usage) < 0) {
        return -1;
    }
    *wall_ms = milliseconds_since(&start);
    return status;
}

static long parse_receiver_output(off_t *offset, FILE *packet_out,
                                  int case_id, unsigned int seed,
                                  int mode, int burst_length, int checktype,
                                  int *validated, int *errors,
                                  int *invalid_metadata)
{
    FILE *log;
    char line[512];
    int packet_index = 0;
    int pending_invalid = 0;
    long bytes = 0;

    *validated = 0;
    *errors = 0;
    *invalid_metadata = 0;
    log = fopen(RECEIVER_LOG, "r");
    if (log == NULL) {
        return -1;
    }
    if (fseeko(log, *offset, SEEK_SET) != 0) {
        fclose(log);
        return -1;
    }
    while (fgets(line, sizeof line, log) != NULL) {
        int result_checktype;
        int detected_error;
        long long time_ns;

        if (strstr(line, "invalid packet metadata:") != NULL) {
            pending_invalid = 1;
        }
        if (sscanf(line, "EVAL_RESULT checktype=%d detected=%d time_ns=%lld",
                   &result_checktype, &detected_error, &time_ns) == 3) {
            (*validated)++;
            if (detected_error) {
                (*errors)++;
            }
            if (pending_invalid) {
                (*invalid_metadata)++;
            }
            fprintf(packet_out,
                    "%d,%d,%u,%s,%d,%s,%d,%d,%d,%d,%lld\n",
                    case_id, packet_index++, seed, scheme_name(checktype),
                    checktype, mode_name(mode), burst_length,
                    result_checktype, detected_error, pending_invalid,
                    time_ns);
            pending_invalid = 0;
        }
    }
    if (fseeko(log, 0, SEEK_END) == 0) {
        off_t end = ftello(log);
        if (end >= 0) {
            bytes = (long)(end - *offset);
            *offset = end;
        }
    }
    fclose(log);
    return bytes;
}

int main(void)
{
    FILE *out;
    FILE *packet_out;
    off_t log_offset = 0;
    int receiver_pid;
    int trial;

    out = fopen(OUTPUT_CSV, "w");
    if (out == NULL) {
        perror(OUTPUT_CSV);
        return EXIT_FAILURE;
    }
    packet_out = fopen(PACKET_OUTPUT_CSV, "w");
    if (packet_out == NULL) {
        perror(PACKET_OUTPUT_CSV);
        fclose(out);
        return EXIT_FAILURE;
    }
    fprintf(packet_out,
            "case_id,packet_index,seed,scheme,checktype,error_mode,burst_length,"
            "receiver_checktype,detected,invalid_metadata,validation_time_ns\n");
    fprintf(out,
            "case_id,trial,seed,scheme,checktype,error_mode,burst_length,validated_packets,"
            "detected_error_packets,invalid_metadata_packets,missed_error_packets,"
            "sender_exit_status,"
            "wall_ms,user_cpu_ms,system_cpu_ms,max_rss_kb,log_bytes\n");

    receiver_pid = start_receiver();
    if (receiver_pid < 0) {
        perror("starting receiver");
        fclose(out);
        return EXIT_FAILURE;
    }

    for (trial = 0; trial < TOTAL_TRIALS; trial++) {
        int case_id = trial / SCHEMES;
        int checktype = trial % SCHEMES;
        int phase = case_id % 10;
        int mode = phase < 1 ? 0 : phase < 3 ? 1 : 2;
        int burst_length = mode == 2 ? 8 + (case_id % 37) : 0;
        unsigned int seed = 8142026U + (unsigned int)case_id;
        int validated;
        int errors;
        int invalid_metadata;
        int status;
        struct rusage usage;
        double wall_ms;
        long log_bytes;
        double user_ms;
        double system_ms;

        status = run_sender(mode, burst_length, checktype, seed,
                            &usage, &wall_ms);
        usleep(10000);
        log_bytes = parse_receiver_output(&log_offset, packet_out,
                                          case_id, seed, mode,
                                          burst_length, checktype,
                                          &validated, &errors,
                                          &invalid_metadata);
        user_ms = usage.ru_utime.tv_sec * 1000.0 +
                  usage.ru_utime.tv_usec / 1000.0;
        system_ms = usage.ru_stime.tv_sec * 1000.0 +
                    usage.ru_stime.tv_usec / 1000.0;

        fprintf(out,
                "%d,%d,%u,%s,%d,%s,%d,%d,%d,%d,%d,%d,%.3f,%.3f,%.3f,%ld,%ld\n",
                case_id + 1, trial + 1, seed, scheme_name(checktype), checktype,
                mode_name(mode),
                burst_length, validated, errors, invalid_metadata,
                mode == 0 ? 0 : validated - errors,
                WIFEXITED(status) ? WEXITSTATUS(status) : -1,
                wall_ms, user_ms, system_ms, usage.ru_maxrss, log_bytes);
        fflush(out);

        if ((trial + 1) % 100 == 0) {
            printf("completed %d/%d trials\n", trial + 1, TOTAL_TRIALS);
        }
    }

    kill((pid_t)receiver_pid, SIGTERM);
    waitpid((pid_t)receiver_pid, NULL, 0);
    fclose(out);
    fclose(packet_out);
    printf("End-to-end evaluation complete: %s and %s\n",
           OUTPUT_CSV, PACKET_OUTPUT_CSV);
    return EXIT_SUCCESS;
}
