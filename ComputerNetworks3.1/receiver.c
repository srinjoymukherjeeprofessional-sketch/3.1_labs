// help and reference: https://beej.us/guide/bgnet/html/split

#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include <sys/types.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netdb.h>
#include <arpa/inet.h>
#include <sys/wait.h>
#include <signal.h>
#include "error_detection.h"

#include "error_detection.h"

#define PORT "3000"
#define BACKLOG 10
#define PACKET_SIZE 64
#define PAYLOADSIZE 44
#define PAYLOADSIZE 44
#define MACSIZE 6


// get sockaddr, IPv4 or IPv6:
void *get_in_addr(struct sockaddr *sa)
{
    if (sa->sa_family == AF_INET) {
        return &(((struct sockaddr_in*)sa)->sin_addr);
    }

    return &(((struct sockaddr_in6*)sa)->sin6_addr);
}

int receive_message(int sock_fd, char* buffer, size_t buffer_size){
    return recv(sock_fd, buffer, buffer_size, 0);
}

int get_client_port(struct sockaddr_storage their_addr){
    int client_port;

    if (their_addr.ss_family == AF_INET) {
        struct sockaddr_in *s = (struct sockaddr_in *)&their_addr;
        client_port = ntohs(s->sin_port); // Extract IPv4 port
    } else {
        struct sockaddr_in6 *s = (struct sockaddr_in6 *)&their_addr;
        client_port = ntohs(s->sin6_port); // Extract IPv6 port
    }
    return client_port;
}

int send_message(int sock_fd, const char* msg){
    return send(sock_fd, msg, strlen(msg), 0);
}

ssize_t recv_all(int fd, char *buffer, size_t size) {
    size_t total = 0;

    while (total < size) {
        ssize_t n = recv(fd, buffer + total, size - total, 0);

        if (n == 0) {
            break;  // sender closed connection
        }
        if (n < 0) {
            return -1;
        }
        total += n;
    }
    return total;
}

void print_hex(const unsigned char *buffer, size_t len) {
    for (size_t i = 0; i < len; i++) {
        printf("%02x ", buffer[i]);

        if ((i + 1) % 16 == 0) {
            printf(" | ");
            for (size_t j = i - 15; j <= i; j++) {
                unsigned char c = buffer[j];
                printf("%c", (c >= 32 && c <= 126) ? c : '.');
            }
            printf("\n");
        }
    }
}


int start_server(){
    
    //A file descriptor (FD) is a non-negative integer that the Unix kernel returns to a process to act as an abstract handle for an open file or input/output (I/O) resource
    // sock_fd holds the file descriptor for listener
    int sock_fd, new_fd;

    // addrinfo is the struct used to store the valid configurations returned by OS for what kind specifications i require for my server, using the method getaddrinfo()
    struct addrinfo hints, *servinfo, *p;//those specs are in hints, valid ones will be in servinfo and p is iterator

    //string text
    const char * text = "Hello from server";
    const char * browsertext = "HTTP/1.1 200 OK\r\n"
    "Content-Type: text/plain\r\n"
    "Content-Length: 17\r\n"
    "Connection: close\r\n"
    "\r\n"
    "Hello from server";

    //define hints struct specify what kind of server we want
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC; // suport both IPv4/6
    hints.ai_socktype = SOCK_STREAM; // stream connection, tcp
    hints.ai_flags = AI_PASSIVE; //binding to 'any' interface

    //catch errors
    int rv;
    if ((rv = getaddrinfo(NULL, PORT, &hints, &servinfo)) != 0) {
        printf("getaddrinfo: %s\n", gai_strerror(rv));
        return 1;
    }

    // loop through all the results and bind to the first we can
    int yes=1;
    for(p = servinfo; p != NULL; p = p->ai_next) {
        if ((sock_fd=socket(p->ai_family, p->ai_socktype, p->ai_protocol) )== -1) {
            printf("listener socket failed\n");
            continue;
        }

        // lose the pesky "Address already in use" error message
        // setsockopt();

        if (bind(sock_fd, p->ai_addr, p->ai_addrlen) == -1) {
            close(sock_fd);
            perror("server: bind"); //perror is a library function that reads and prints the global 'errno' variable
            continue;
        }
        break;
    }
    freeaddrinfo(servinfo); // all done with this structure

    if (p==NULL) {
        printf("server: failed to bind\n");
        return 2;
    }

    //
    if (listen(sock_fd, BACKLOG) == -1) {
        perror("listen");
        return 3;
    }
    printf("server: listening for connections...\n");


    struct sockaddr_storage their_addr; //This is where the information about the incoming connection will go (and with it you can determine which host is calling you from which port)
    socklen_t sin_size = sizeof their_addr;
    char s[INET6_ADDRSTRLEN];//will store sender details, space for ipv6 format

    int iterations=1;
    //infinite loop to keep server listening
    while(1){
        // int accept(int sock_fd, struct sockaddr *addr, socklen_t *addrlen);

        printf("%d th time \t",iterations);
        new_fd = accept(sock_fd, (struct sockaddr *)&their_addr, &sin_size);
        if (new_fd == -1) {
            perror("accept");
            break;
        }

        //hold sender port
        unsigned short client_port;
        client_port = get_client_port(their_addr);

        // Stands for "Network to Presentation". It reads the binary IP stored inside their_addr and writes the text representation into the string buffer s.
        inet_ntop(their_addr.ss_family,
            get_in_addr((struct sockaddr *)&their_addr),
            s, sizeof s);
        printf("server: got connection from %s:%d\n", s,client_port);

        // send_message(new_fd, text);


        char buffer[PACKET_SIZE];
        ssize_t bytes_received;

        int bytes_received = 0;
        int all_bytes_good=1;
        while ((bytes_received = recv_all(new_fd, buffer, PACKET_SIZE)) > 0) {

            if (bytes_received == PACKET_SIZE) {
                unsigned short received_checksum;
                unsigned short calculated_checksum;

                received_checksum = ((unsigned char)buffer[62] << 8) |
                                    (unsigned char)buffer[63];
                calculated_checksum = checksum(buffer, 2*MACSIZE+sizeof(int)+PAYLOADSIZE);

                if (calculated_checksum == received_checksum) {
                    printf("checksum: OK\n");
                } else {
                    printf("checksum: ERROR\n");
                    printf("calculated: %04X, received: %04X\n",
                           calculated_checksum, received_checksum);
                    all_bytes_good=0;
                }
            }

            printf("server: received %zd bytes\n", bytes_received);
            print_hex((char *)buffer, bytes_received);

            printf("\n");
            fflush(stdout);
            memset(buffer, 0, sizeof buffer);

            if (bytes_received < PACKET_SIZE) {
                break;  // final partial block
            }
        }


        close(new_fd);//close FD
        printf("server: conn closed\n");
        iterations++;
    }
    return 0;
}

int main()
{
    start_server();
    return 0;
}
