#!/bin/bash

if([ $# != 1 ]); then
    echo "Usage: $0 <directory_name>"
    exit 1
fi
DIRNAME=$1

count_all=$(find "$DIRNAME" | wc -l)

list_files=$(find "$DIRNAME" -type f)
count_files=$(list_files | wc -l)

list_dirs=$(find "$DIRNAME" -type d)
count_dirs=$(list_dirs | wc -l)


echo "Total number of files and directories in '$DIRNAME': $count_all"

echo "files in '$DIRNAME':"
echo "$list_files"
echo "Total number of files in '$DIRNAME': $count_files"


echo "directories in '$DIRNAME':"
echo "$list_dirs"
echo "Total number of directories in '$DIRNAME': $count_dirs"