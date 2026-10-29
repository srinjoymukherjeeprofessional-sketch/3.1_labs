#include <stdio.h>

int main()
{
    int n, i, index, blocks[50];

    printf("Enter the index block number: ");
    scanf("%d", &index);

    printf("Enter the number of blocks required: ");
    scanf("%d", &n);

    printf("Enter the block numbers allocated to the file:\n");
    for (i = 0; i < n; i++)
    {
        scanf("%d", &blocks[i]);
    }

    printf("\nIndexed File Allocation\n");
    printf("----------------------\n");
    printf("Index Block: %d\n", index);
    printf("File Block\tAllocated Block\n");

    for (i = 0; i < n; i++)
    {
        printf("%d\t\t%d\n", i + 1, blocks[i]);
    }

    printf("\nFile allocation completed successfully.\n");

    return 0;
}