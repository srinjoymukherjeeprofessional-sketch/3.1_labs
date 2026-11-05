int pages[] = {
    9, 10, 11, 7, 12, 8, 7, 6, 12, 5,
    4, 3, 10, 11, 12, 4, 5, 6, 9, 4, 5
};

int n= sizeof(pages)/sizeof(pages[0]);

void fifo(int capacity, int pages[], int n){
    int queue[capacity];
    int back=0, front=0;
    for (int i=0;i<n;i++){
        int pagenum=pages[i];
        if (front>=capacity-1){
            
        }
    }
}