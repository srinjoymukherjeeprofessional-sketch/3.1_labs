#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <sys/types.h>
#include <sys/wait.h>

int main()
{
    int pipeXY[2];   
    int pipeYX[2];   

    if (pipe(pipeXY) == -1) {
        perror("pipeXY");
        exit(1);
    }

    if (pipe(pipeYX) == -1) {
        perror("pipeYX");
        exit(1);
    }

    pid_t p = fork();

    if (p < 0) {
        perror("fork");
        exit(1);
    }

    if (p == 0) {
        close(pipeXY[1]);  
        close(pipeYX[0]); 

        int x;
        int y = 2;

        read(pipeXY[0], &x, sizeof(x));

        printf("Y: received x = %d\n", x);

        printf("Y: writing y = %d\n", y);

        write(pipeYX[1], &y, sizeof(y));

        close(pipeXY[0]);
        close(pipeYX[1]);

        exit(0);

    } else {
        close(pipeXY[0]);
        close(pipeYX[1]);  

        int x = 1;
        int y;

        printf("X: writing x = %d\n", x);

        write(pipeXY[1], &x, sizeof(x));

        read(pipeYX[0], &y, sizeof(y));

        printf("X: received y = %d\n", y);

        close(pipeXY[1]);
        close(pipeYX[0]);

        waitpid(p, NULL, 0);

        printf("exiting \n");
    }

    return 0;
}
