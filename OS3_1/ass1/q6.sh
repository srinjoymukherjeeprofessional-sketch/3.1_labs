#!/bin/bash

echo "Enter a file name: "
read filename

echo "Enter a string to search for: "
read str

# check file exists
if [ ! -f "$filename" ]; then
    echo "File '$filename' does not exist."
    exit 1
fi

count=$(grep -o "$str" "$filename" | wc -l)

echo "The string '$str' appears $count times in the file '$filename'."

if ([ "$count" == 0 ]); then
    echo "exiting..."
    exit 0
fi

line=1
while read -r curline; do
    cnt=$(echo "$curline" | grep -o "$str" | wc -l)
    if [ "$cnt" -gt 0 ]; then
        echo "Line $line: $cnt occurrences"
    fi
    line=$((line + 1))
done < "$filename"