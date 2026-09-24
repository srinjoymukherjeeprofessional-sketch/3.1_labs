#include<stdio.h>
#include<stdlib.h>

#include<sys/types.h>
#include<unistd.h>

#include<sys/wait.h>
#include<fcntl.h>


#define FIFO1 "sender_to_receiver"
#define FIFO2 "receiver_to_sender"
#define buf 256
#define ITERS 5

int main(){
    mkfifo(FIFO1, 0666);
    mkfifo(FIFO2, 0666);
    
    int fd1,fd2;
    //read
    fd1=open(FIFO1, O_RDONLY);
    //write
    fd2=open(FIFO2, O_WRONLY);
    
    char message[buf];

    pid_t p=fork();
    //simultaneous receive and send
    if (p==0){
        int iter=ITERS;
        while(iter){
            //read
            read(fd1,message, buf);
            printf("received: %s", buf);
            iter--;
        }
    }else{
        int iter=ITERS;
        while(iter){
            //write
            char outmessage[buf];
            printf("Enter message to send\n");
            scanf("%s", outmessage);
            write(fd2, outmessage, buf);
            iter--;
        }
        printf("writing complete\n");
        close(fd2);
        wait(p);
        close(fd1);
    }
}
