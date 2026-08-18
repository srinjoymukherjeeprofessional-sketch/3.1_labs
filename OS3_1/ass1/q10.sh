#!/bin/bash

if ([ $# -lt 1 ]); then
    echo "Usage: $0 <file(optional)> -cl(optional)"
    exit 1
fi

BIN="my-deleted-files"
if ([ "$1" == "-cl" ]); then
    rm -r "$BIN"
    echo "Deleted '$BIN' directory and its contents."
    exit 0
fi

file=$1
mkdir -p "$BIN"

if ([ ! -f "$file" ]); then
    echo "Error: '$file' is not present"
    exit 1
fi

# parenthese around a string turns it into array, division by space
status=($(find "./$BIN" -maxdepth 1 -name "${file}*" ))
# echo $status
if ([ ${#status[@]} -eq 0 ]); then
    mv "$file" "$BIN/"
    exit 0
fi
echo "Found ${#status[@]} matching file(s) in $BIN:"

# Loop through the array to inspect each matching file
count=0
# math expressions need to be inside (())
for f in "${status[@]}"; do
    ((count+=1))
done

newfile="${file}($((count-1)))"

mv "$file" "$BIN/$newfile"