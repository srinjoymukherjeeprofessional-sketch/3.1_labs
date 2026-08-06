#!/bin/bash

FILE1=$1
FILE2=$2
FILE3=$3
FILE4=$4

WORD1="public"
WORD2="class"
WORD3="int"

if ([ $# -ne 4 ]); then
echo "usage: ./q5.sh file1 file2 file3 file4"
exit 0
fi

countword(){
    local word=$1
    count1=$(grep -c -w "$word" $FILE1)
    count2=$(grep -c -w "$word" $FILE2)
    count3=$(grep -c -w "$word" $FILE3)
    count4=$(grep -c -w "$word" $FILE4)
    echo "counts of word in files:"
    echo "$FILE1 -> $count1"
    echo "$FILE2 -> $count2"
    echo "$FILE3 -> $count3"
    echo "$FILE4 -> $count4"
}

echo "$(countword "$WORD1")"
echo "$(countword "$WORD3")"
echo "$(countword "$WORD3")"
