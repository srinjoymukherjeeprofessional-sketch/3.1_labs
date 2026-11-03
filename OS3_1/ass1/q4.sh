#!/bin/bash

if([ $# != 1 ]); then
    echo "Usage: $0 <directory_name>"
    exit 1
fi
DIRNAME=$1

total_files=$(find "$DIRNAME" -type f | wc -l)

echo "Total number of files in '$DIRNAME' and all subdirectories: $total_files"

echo "File counts per subdirectory:"
find "$DIRNAME" -type d | while read -r dir; do
    # Count regular files directly inside this specific directory (not recursive)
    dir_file_count=$(find "$dir" -maxdepth 1 -type f | wc -l)
    echo "Directory: $dir | Files: $dir_file_count"
done

echo "Files created/modified within the past week:"
recent_files=$(find "$DIRNAME" -type f -mtime -7)

if [ -n "$recent_files" ]; then
    echo "$recent_files"
else
    echo "No files created/modified in the past week."
fi