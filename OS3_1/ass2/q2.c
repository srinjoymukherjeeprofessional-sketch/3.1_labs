#include<stdio.h>
#include<stdlib.h>

//usleep
#include<time.h>

//fork
#include<sys/types.h>
#include<unistd.h>

//waitpid
#include<sys/wait.h>

int main(){
    int n1, n2;
    printf("Enter number of iterations for child 1\n");
    fflush(stdout);
    scanf("%d",&n1);

    printf("Enter number of iterations for child 2\n");
    fflush(stdout);
    scanf("%d",&n2);

    //pipe 
    int fd[2];
    if(pipe(fd)<0){
        perror("pipe");
        exit(1);
    }

    pid_t p1 = fork();

    if (p1<0){
        perror("fork");
        exit(1);
    }
    if (p1==0){
        //child process X
        printf("process name:%d\n", (int)getpid());
            
        for(int i=0;i<n1;i++){
            int sleeptime = rand()%20;
            printf("child 1 %d\n", i);
            //write to the file
            write(fd[1],"1",1);
            printf("child 1 sleeping for %d ms\n", sleeptime);
            usleep(100000*sleeptime);
        }
        printf("child %d exiting.... \n", getpid());
        exit(0);
    }else{
        //parent process
        pid_t p2 = fork();
        if (p2<0){
            perror("fork");
            exit(1);
        }
        if (p2==0){
            //child process Y
            printf("process name:%d\n", (int)getpid());
            
            for(int i=0;i<n2;i++){
                if (i) {
                    char buf;
                    read(fd[0],&buf,1);
                }
                int sleeptime = rand()%10;
                printf("child 2 %d\n", i);
                printf("child 2 sleeping for %d ms\n", sleeptime);
                usleep(100000*sleeptime);
            }
            printf("child %d exiting....\n",getpid());
            exit(0);    
        }
        else{
            //parent process
            waitpid(p1, NULL, 0);
            waitpid(p2, NULL, 0);
            printf("parent exiting...\n");
            exit(0);
        }
    }
}