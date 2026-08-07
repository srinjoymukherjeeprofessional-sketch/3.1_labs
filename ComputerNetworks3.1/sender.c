// help and reference: https://beej.us/guide/bgnet/html/split

#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <errno.h>
#include <string.h>
#include <netdb.h>
#include <sys/types.h>
#include <netinet/in.h>
#include <sys/socket.h>
#include <arpa/inet.h>
#include "error_detection.h"
#include "error_injection.h"

char* PORT = "3000"; // the port client will be connecting to 

#define PAYLOADSIZE 44 // max number of bytes we can get at once 
#define FILENAME "text.txt"
#define PACKET_SIZE 64 // Same as server packet size
#define MACSIZE 6	

char* make_packet(const char* buffer, int payload_len);

// get sockaddr, IPv4 or IPv6:
void *get_in_addr(struct sockaddr *sa)
{
	if (sa->sa_family == AF_INET) {
		return &(((struct sockaddr_in*)sa)->sin_addr);
	}

	return &(((struct sockaddr_in6*)sa)->sin6_addr);
}

// reads a file and sends it in packets of size PAYLOADSIZE
int send_file_in_packets(int sockfd, const char* filename,
                         int mode, int burst_length) {
    FILE* file = fopen(filename, "rb");
    if (file == NULL) {
        perror("Failed to open file");
        return -1;
    }

    char buffer[PAYLOADSIZE];
    size_t bytes_read;
	int packets = 0;
    // Read and send loop: 64 bytes at a time
    while ((bytes_read = fread(buffer, 1, PAYLOADSIZE, file)) > 0) {
        int total_sent = 0;
		
		char* payload;
		payload = make_packet(buffer, bytes_read);
		if (payload == NULL) {
			fprintf(stderr, "failed to create packet\n");
			fclose(file);
			return -1;
		}

		// The checksum is already inside payload. Inject the error only now,
		// so the receiver can detect the change.
		if (mode == 1) {
			inject_random_position(payload, PACKET_SIZE);
		} else if (mode == 2) {
			inject_burst(payload, PACKET_SIZE, burst_length);
		}

	        // Ensure all bytes of the current packet are sent
        while (total_sent < PACKET_SIZE) {
            int bytes_sent = send(sockfd, payload + total_sent, PACKET_SIZE - total_sent, 0);
            if (bytes_sent == -1) {
                perror("send failed");
                fclose(file);
                return -1;
            }
            total_sent += bytes_sent;
        }
		free(payload);
		packets++;
    }

    if (ferror(file)) {
        perror("Error reading file");
        fclose(file);
        return -1;
    }

    fclose(file);
    printf("File sent successfully in packets (%d total).\n", packets);
    return 0;
}

//buffer contains the payload
char* make_packet(const char* buffer, int payload_len)
{
    char dest_mac[MACSIZE] = {'A','A','B','B','C','C'};
    char src_mac[MACSIZE]  = {'X','Y','Z','Y','Z','X'};

    char *packet = (char *)malloc(PACKET_SIZE);
    memset(packet, 0, PACKET_SIZE); // zero the padding tail
    char *ptr = packet;//just copy 

    memcpy(ptr, dest_mac, MACSIZE);         ptr += MACSIZE;
    memcpy(ptr, src_mac, MACSIZE);          ptr += MACSIZE;
    memcpy(ptr, &payload_len, sizeof(int)); ptr += sizeof(int);
    memcpy(ptr, buffer, payload_len);       ptr += PAYLOADSIZE;

    unsigned short cs;

    // memcpy(padded_payload, buffer, payload_len);
    cs = checksum(packet, 2*MACSIZE+sizeof(int)+PAYLOADSIZE);


    // big endian checksum: 00 00 CC CC.
    ptr[0] = 0;
    ptr[1] = 0;
    ptr[2] = (char)((cs >> 8) & 0xff);
    ptr[3] = (char)(cs & 0xff);

    return packet;
}

int send_message(int sock_fd, const char* msg){
	// send() returns the number of bytes actually sent out—this might be less than the number you told it to send!
	return send(sock_fd, msg, strlen(msg), 0);
}

int main(int argc, char *argv[])
{
	int sockfd, numbytes;  
	int mode;
	int burst_length = 0;
	char buf[PAYLOADSIZE];
	struct addrinfo hints, *servinfo, *p;
	int rv;
	char s[INET6_ADDRSTRLEN];

	if (argc != 2 && argc != 3) {
	    fprintf(stderr,"usage: client <hostname/default 3000>\n");
	    exit(1);
	}

	printf("Choose error injection mode:\n");
	printf("0 = no error\n");
	printf("1 = random single-byte error\n");
	printf("2 = burst error\n");
	printf("Enter mode: ");
	if (scanf("%d", &mode) != 1 ||
	    mode < 0 || mode > 2) {
		fprintf(stderr, "Invalid injection mode\n");
		return 1;
	}

	if (mode == 2) {
		printf("Enter burst length in bytes (1-%d): ", PACKET_SIZE);
		if (scanf("%d", &burst_length) != 1 ||
		    burst_length < 1 || burst_length > PACKET_SIZE) {
			fprintf(stderr, "Invalid burst length\n");
			return 1;
		}
	}

    //define hints struct specify what kind of server we want
    //works fine without hints too in this case 
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_UNSPEC; // use IPv4 or IPv6
	hints.ai_socktype = SOCK_STREAM; //tcp

    if (argc == 3) {
        PORT = argv[2];
    }

	if ((rv = getaddrinfo(argv[1], (const char *)PORT, &hints, &servinfo)) != 0) {
		fprintf(stderr, "getaddrinfo: %s\n", gai_strerror(rv));
		return 1;
	}

	// loop through all the results and connect to the first we can
	for(p = servinfo; p != NULL; p = p->ai_next) {
		if ((sockfd = socket(p->ai_family, p->ai_socktype,
				p->ai_protocol)) == -1) {
			perror("client: socket");
			continue;
		}

        inet_ntop(p->ai_family,
            get_in_addr((struct sockaddr *)p->ai_addr),
            s, sizeof s);
        printf("client: attempting connection to %s\n", s);

		if (connect(sockfd, p->ai_addr, p->ai_addrlen) == -1) {
			perror("client: connect");
			close(sockfd);
			continue;
		}
		
		break;
	}

	if (p == NULL) {
		fprintf(stderr, "client: failed to connect\n");
		return 2;
	}

	inet_ntop(p->ai_family,
			get_in_addr((struct sockaddr *)p->ai_addr),
			s, sizeof s);
	printf("client: connected to %s\n", s);

	freeaddrinfo(servinfo); // all done with this structure

	// if ((numbytes = recv(sockfd, buf, MAXDATASIZE-1, 0)) == -1) {
	//     perror("recv");
	//     exit(1);
	// }

	// buf[numbytes] = '\0';

	// printf("client: received '%s'\n",buf);

	const char* filename = FILENAME; // Specify your file name here
	seed_error_injection();
	if (send_file_in_packets(sockfd, filename, mode, burst_length) == -1) {
		close(sockfd);
		exit(1);
	}

	close(sockfd);

	return 0;
}
