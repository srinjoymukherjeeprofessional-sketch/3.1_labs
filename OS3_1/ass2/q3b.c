#include<stdio.h>
#include<stdlib.h>

#include<sys/types.h>
#include<unistd.h>

#include<sys/wait.h>

int main(){
    int pipesr[2], pipers[2];
    if (pipe(pipesr)<0){
        perror(pipe);
        exit (1);
    }
    if (pipe(pipers<0)){
        perror("pipe");
        exit (1);
    }
    pid_t p=fork();
    if (p==0){
        //child process
        
    }

}
