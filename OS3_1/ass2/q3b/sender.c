#include<stdio.h>
#include<stdlib.h>

#include<sys/types.h>
#include<unistd.h>

#include<sys/wait.h>

#include <sys/stat.h>
#include<fcntl.h>


#define FIFO1 "sender_to_receiver"
#define FIFO2 "receiver_to_sender"
#define buf 256
#define ITERS 5

int main(){
    mkfifo(FIFO1, 0666);
    mkfifo(FIFO2, 0666);
    
    int fd1,fd2;
    //write
    fd1=open(FIFO1, O_WRONLY);
    //read
    fd2=open(FIFO2, O_RDONLY);
    
    char message[buf];

    pid_t p=fork();
    //simultaneous receive and send
    if (p==0){
        int iter=ITERS;
        while(iter){
            //read
            read(fd2,message, buf);
            printf("received: %s", message);
            iter--;
        }
    }else{
        int iter=ITERS;
        while(iter){
            //write
            char outmessage[buf];
            printf("Enter message to send\n");
            scanf("%s", outmessage);
            write(fd1, outmessage, buf);
            iter--;
        }
        printf("writing complete\n");
        close(fd2);
        wait(p);
        close(fd1);
    }
}
