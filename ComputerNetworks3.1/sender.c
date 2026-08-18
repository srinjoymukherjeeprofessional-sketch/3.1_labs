// help and reference: https://beej.us/guide/bgnet/html/split

#define _POSIX_C_SOURCE 200112L

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
#include <stdint.h>
#include "error_detection.h"
#include "error_injection.h"

char* PORT = "3000"; // the port client will be connecting to 

#define PAYLOADSIZE 44 // max number of bytes we can get at once 
#define FILENAME "text.txt"
#define PACKET_SIZE 64 // Same as server packet size
#define MACSIZE 6	

char* make_packet(const char* buffer, int payload_len, int checktype);

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
                         int mode, int burst_length, int checktype) {
    FILE* file = fopen(filename, "rb");
    const char *limit_text;
    int injection_size = PACKET_SIZE;
    if (file == NULL) {
        perror("Failed to open file");
        return -1;
    }

    limit_text = getenv("ERROR_INJECTION_LIMIT");
    if (limit_text != NULL) {
        char *end = NULL;
        long requested_limit = strtol(limit_text, &end, 10);

        if (end != limit_text && *end == '\0' &&
            requested_limit > 0 && requested_limit <= PACKET_SIZE) {
            injection_size = (int)requested_limit;
        }
    }
    if (mode == 2 && burst_length > injection_size) {
        fprintf(stderr, "Burst length exceeds evaluation injection limit\n");
        fclose(file);
        return -1;
    }

    char buffer[PAYLOADSIZE];
    size_t bytes_read;
	int packets = 0;
    int evaluation_detail = getenv("EVAL_DETAIL") != NULL;
    // Read and send loop: 64 bytes at a time
    while ((bytes_read = fread(buffer, 1, PAYLOADSIZE, file)) > 0) {
        int total_sent = 0;
		
		char* payload;
		payload = make_packet(buffer, bytes_read, checktype);
		if (payload == NULL) {
			fprintf(stderr, "failed to create packet\n");
			fclose(file);
			return -1;
		}

		// The checksum is already inside payload. Inject the error only now,
		// so the receiver can detect the change.
		if (mode == 1) {
			inject_random_position(payload, injection_size);
		} else if (mode == 2) {
			inject_burst(payload, injection_size, burst_length);
		}
		if (evaluation_detail) {
			printf("EVAL_ERROR packet=%d start=%d length=%d\n",
			       packets, mode == 0 ? -1 : last_error_position(),
			       mode == 0 ? 0 : last_error_length());
			fflush(stdout);
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
char* make_packet(const char* buffer, int payload_len, int checktype)
{
    char dest_mac[MACSIZE] = {'A','A','B','B','C','C'};
    char src_mac[MACSIZE]  = {'X','Y','Z','Y','Z','X'};

    char *packet = (char *)malloc(PACKET_SIZE);
    memset(packet, 0, PACKET_SIZE); // zero the padding tail
    char *ptr = packet;//just copy 

    memcpy(ptr, dest_mac, MACSIZE);         ptr += MACSIZE;
    memcpy(ptr, src_mac, MACSIZE);          ptr += MACSIZE;
    uint32_t header = htonl(((uint32_t)checktype << 24) |
                            (uint32_t)payload_len);
    memcpy(ptr, &header, sizeof header);    ptr += sizeof header;
    memcpy(ptr, buffer, payload_len);       ptr += PAYLOADSIZE;

	if (checktype==0){
		unsigned short cs;

		// memcpy(padded_payload, buffer, payload_len);
		cs = checksum(packet, 2*MACSIZE+sizeof(int)+PAYLOADSIZE);


		// big endian checksum: 00 00 CC CC.
		ptr[0] = 0;
		ptr[1] = 0;
		ptr[2] = (char)((cs >> 8) & 0xff);
		ptr[3] = (char)(cs & 0xff);
	}else if (checktype==1){
		unsigned short crc;

		crc = crc16(packet, 2*MACSIZE+sizeof(int)+PAYLOADSIZE);

		// big endian
		ptr[0] = 0;
		ptr[1] = 0;
		ptr[2] = (char)((crc >> 8) & 0xff);
		ptr[3] = (char)(crc & 0xff);
	}else if (checktype==2){
		unsigned int crc;

		crc = crc32(packet, 2*MACSIZE+sizeof(int)+PAYLOADSIZE);

		ptr[0]=(char)((crc >> 24) & 0xff);
		ptr[1]=(char)((crc >> 16) & 0xff);
		ptr[2]=(char)((crc >> 8) & 0xff);
		ptr[3]=(char)(crc & 0xff);
	}else if (checktype==3){
		unsigned short crc = crc10(packet, 2*MACSIZE+sizeof(header)+PAYLOADSIZE);

		ptr[0] = 0;
		ptr[1] = 0;
		ptr[2] = (char)((crc >> 8) & 0xff);
		ptr[3] = (char)(crc & 0xff);
	}else if (checktype==4){
		unsigned char crc = crc8(packet, 2*MACSIZE+sizeof(header)+PAYLOADSIZE);

		ptr[0] = 0;
		ptr[1] = 0;
		ptr[2] = 0;
		ptr[3] = (char)crc;
	}

    return packet;
}

int send_message(int sock_fd, const char* msg){
	// send() returns the number of bytes actually sent out—this might be less than the number you told it to send!
	return send(sock_fd, msg, strlen(msg), 0);
}

int main(int argc, char *argv[])
{
	int sockfd, numbytes;  
	int mode, checktype;
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

	//type of error detection
	printf("Choose error detection type:\n");
	printf("0 = checksum\n");
	printf("1 = CRC-16\n");
	printf("2 = CRC-32\n");
	printf("3 = CRC-10 (ATM, polynomial 0x233)\n");
	printf("4 = CRC-8 (ATM, polynomial 0x07)\n");

	if (scanf("%d", &checktype) != 1 || checktype < 0 || checktype > 4) {
		fprintf(stderr, "Invalid error detection type\n");
		return 1;
	}

    //define hints struct specify what kind of server we want
    //works fine without hints too in this case 
	memset(&hints, 0, sizeof hints);
	hints.ai_family = AF_UNSPEC; // use IPv4 or IPv6
	hints.ai_socktype = SOCK_STREAM; //tcp

    if (argc == 3) {
        PORT = argv[2];
    }
/*	getaddrinfo translates a URL/hostname (argv[1]) or 
IP address into a list of remote destination socket 
addresses using the OS name service.*/
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
	{
		const char *evaluation_seed = getenv("ERROR_INJECTION_SEED");
		if (evaluation_seed != NULL) {
			seed_error_injection_value((unsigned int)strtoul(evaluation_seed, NULL, 10));
		} else {
			seed_error_injection();
		}
	}
	if (send_file_in_packets(sockfd, filename, mode, burst_length, checktype) == -1) {
		close(sockfd);
		exit(1);
	}

	/* Tell the receiver that no more packet bytes will be sent.  Without this half-close, recv_all() on the receiver keeps waiting for the next 64-byte packet, while we wait for its response below. */
	if (shutdown(sockfd, SHUT_WR) == -1) {
		perror("shutdown");
		close(sockfd);
		return 1;
	}

	char server_msg[PAYLOADSIZE];
	ssize_t reply_bytes = recv(sockfd, server_msg, sizeof(server_msg) - 1, 0);
	if (reply_bytes < 0) {
		perror("recv reply");
		close(sockfd);
		return 1;
	}
	server_msg[reply_bytes] = '\0';
	printf("received: '%s'\n", server_msg);

	close(sockfd);

	return 0;
}
