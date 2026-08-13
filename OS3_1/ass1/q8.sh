#!/bin/bash

read -p "enter first birthday YYYY/MM/DD: " bd1
read -p "enter second birthday YYYY/MM/DD: " bd2

if ([ "$bd1" = '' ]); then
    bd1="1970/01/01"
fi

if ([ "$bd2" = '' ]); then
    bd2="1971/01/01"
fi

# %A     locale's full weekday name (e.g., Sunday) 
day1=$(date -d "$bd1" +%A)
day2=$(date -d "$bd2" +%A)

if ([ "$day1" == "$day2" ]); then
    echo "Both days are same"
else
    echo "Days are different"
fi