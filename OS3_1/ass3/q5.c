#include <stdio.h>

int main()
{
    int n, i, blocks[50];

    printf("Enter the number of blocks required: ");
    scanf("%d", &n);

    printf("Enter the block numbers allocated to the file:\n");
    for (i = 0; i < n; i++)
    {
        scanf("%d", &blocks[i]);
    }

    printf("\nLinked File Allocation\n");
    printf("----------------------\n");

    for (i = 0; i < n; i++)
    {
        if (i < n - 1)
            printf("%d -> ", blocks[i]);
        else
            printf("%d -> NULL\n", blocks[i]);
    }

    printf("\nFile allocation completed successfully.\n");

    return 0;
}
