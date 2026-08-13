#!/bin/bash

if([ $# != 1 ]); then
    echo "Usage: $0 <directory_name>"
    exit 1
fi

DIRNAME=$1

echo "List of files in $DIRNAME:"
echo (find "$DIRNAME" -file)