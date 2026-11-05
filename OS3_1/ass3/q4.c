#include<stdio.h>

void lfu(int capacity, int pages[], int n) {
    int frames[capacity];
    int frequency[capacity];
    int lastUsed[capacity];

    int time = 0;
    int count = 0;
    int faults = 0, hits = 0;

    // Initialize arrays
    for (int i = 0; i < capacity; i++) {
        frames[i] = -1;
        frequency[i] = 0;
        lastUsed[i] = 0;
    }

    printf("\n--- LFU Page Replacement (%d Frames) ---\n", capacity);
    printf("Page\tFrames\t\t\tStatus\n");

    for (int i = 0; i < n; i++) {
        time++;

        int found = -1;

        // Search for the page
        for (int j = 0; j < count; j++) {
            if (frames[j] == pages[i]) {
                found = j;
                break;
            }
        }

        if (found != -1) {
            // Page Hit
            hits++;

            frequency[found]++;
            lastUsed[found] = time;
        }
        else {
            // Page Fault
            faults++;

            int victim = -1;

            // Find empty frame
            if (count < capacity) {
                victim = count;
                count++;
            }
            else {
                // Find LFU page
                victim = 0;

                for (int j = 1; j < capacity; j++) {

                    // Lower frequency means higher priority for replacement
                    if (frequency[j] < frequency[victim]) {
                        victim = j;
                    }

                    // If frequencies are equal, replace LRU page
                    else if (frequency[j] == frequency[victim] &&
                            lastUsed[j] < lastUsed[victim]) {
                        victim = j;
                    }
                }
            }

            // Insert new page
            frames[victim] = pages[i];
            frequency[victim] = 1;
            lastUsed[victim] = time;
        }

        // Display frames
        printf("%d\t", pages[i]);

        for (int j = 0; j < capacity; j++) {
            if (frames[j] == -1)
                printf("- ");
            else
                printf("%d ", frames[j]);
        }

        if (found != -1)
            printf("\tHit\n");
        else
            printf("\tFault\n");
    }

    printf("\nTotal Page Faults: %d\n", faults);
    printf("Total Page Hits: %d\n", hits);
}

int main(){
    int pages[] = {9, 10, 11, 7, 12, 8, 7, 6, 12, 5, 4, 3, 10, 11, 12, 4, 5, 6, 9, 4, 5};
    lfu(4, pages, 21);
    lfu(7, pages, 21);
    return 0;
}