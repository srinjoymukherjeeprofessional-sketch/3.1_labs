
#include <stdio.h>

int pages[] = {
    9, 10, 11, 7, 12, 8, 7, 6, 12, 5,
    4, 3, 10, 11, 12, 4, 5, 6, 9, 4, 5
};

int n = sizeof(pages) / sizeof(pages[0]);

// METHOD 1: Timestamp method

void lruTimestamp(int capacity) {
    int frames[capacity];
    int lastUsed[capacity];

    int time = 0;
    int faults = 0, hits = 0;

    for (int i = 0; i < capacity; i++) {
        frames[i] = -1;
        lastUsed[i] = -1;
    }

    printf("\n--- LRU Timestamp Method (%d Frames) ---\n", capacity);
    printf("Page\tFrames\t\t\tStatus\n");

    for (int i = 0; i < n; i++) {
        time++;

        int found = -1;

        // Search for the page
        for (int j = 0; j < capacity; j++) {
            if (frames[j] == pages[i]) {
                found = j;
                break;
            }
        }

        if (found != -1) {
            // Page hit
            hits++;
            lastUsed[found] = time;
        }
        else {
            // Page fault
            faults++;

            int victim = -1;

            // Find an empty frame first
            for (int j = 0; j < capacity; j++) {
                if (frames[j] == -1) {
                    victim = j;
                    break;
                }
            }

            // If no empty frame, find LRU page
            if (victim == -1) {
                victim = 0;

                for (int j = 1; j < capacity; j++) {
                    if (lastUsed[j] < lastUsed[victim]) {
                        victim = j;
                    }
                }
            }

            frames[victim] = pages[i];
            lastUsed[victim] = time;
        }

        // Display current frames
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


// METHOD 2: Recency ordering method

void lruOrdering(int capacity) {
    int frames[capacity];

    int count = 0;
    int faults = 0, hits = 0;

    printf("\n--- LRU Ordering Method (%d Frames) ---\n", capacity);
    printf("Page\tLRU -> MRU\t\tStatus\n");

    for (int i = 0; i < n; i++) {
        int pos = -1;

        // Search for the page
        for (int j = 0; j < count; j++) {
            if (frames[j] == pages[i]) {
                pos = j;
                break;
            }
        }

        if (pos != -1) {
            // Page hit: remove from current position
            hits++;

            int temp = frames[pos];

            for (int j = pos; j < count - 1; j++) {
                frames[j] = frames[j + 1];
            }

            // Move accessed page to MRU position
            frames[count - 1] = temp;
        }
        else {
            // Page fault
            faults++;

            if (count < capacity) {
                // Empty frame available
                frames[count] = pages[i];
                count++;
            }
            else {
                // Remove LRU page (first element)
                for (int j = 0; j < count - 1; j++) {
                    frames[j] = frames[j + 1];
                }

                // Insert new page at MRU position
                frames[count - 1] = pages[i];
            }
        }

        // Display current recency order
        printf("%d\t", pages[i]);

        for (int j = 0; j < count; j++) {
            printf("%d ", frames[j]);
        }

        if (pos != -1)
            printf("\tHit\n");
        else
            printf("\tFault\n");
    }

    printf("\nTotal Page Faults: %d\n", faults);
    printf("Total Page Hits: %d\n", hits);
}


int main() {
    printf("LRU PAGE REPLACEMENT SIMULATION\n");

    // Method 1: Timestamp
    lruTimestamp(5);
    lruTimestamp(6);

    // Method 2: Recency ordering
    lruOrdering(5);
    lruOrdering(6);

    return 0;
}
