#!/bin/bash

while true; do
 
    read -p "Enter first variable (userv1): " userv1
    read -p "Enter second variable (userv2): " userv2
    echo "\n"

    # Regular expression pattern to check for valid numbers (integer or real/float)
    numeric_pattern='^[-+]?[0-9]+([.][0-9]+)?$'

    # Validate if both inputs are numeric
    if [[ $userv1 =~ $numeric_pattern ]] && [[ $userv2 =~ $numeric_pattern ]]; then
        # (a) Addition
        sum=$(awk "BEGIN {print $userv1 + $userv2}")
        echo "The sum of '$userv1' and '$userv2' is $sum"

        # (b) Multiplication
        prod=$(awk "BEGIN {print $userv1 * $userv2}")
        echo "The product of '$userv1' and '$userv2' is $prod"

        # (c) Subtraction
        diff=$(awk "BEGIN {print $userv1 - $userv2}")
        echo "The difference of '$userv1' and '$userv2' is $diff"

        # (d) Division
        is_zero=$(awk "BEGIN {print ($userv2 == 0) ? 1 : 0}")
        if [ "$is_zero" -eq 1 ]; then
            echo "Error: Division cannot be performed (division by zero)."
        else
            div=$(awk "BEGIN {printf \"%.4f\", $userv1 / $userv2}")
            echo "The division of '$userv1' by '$userv2' is $div"
        fi
    else
        echo "Error: Arithmetic operations (a-d) cannot be performed because one or both variables are not numeric."
    fi

    # (e) Printing in reverse order 
    echo "The variables in reverse order are: '$userv2' and '$userv1'"

    read -p "Do you want to execute again? (y/n): " choice
    if [[ "$choice" =~ ^[Nn]$ ]]; then
        echo "Exiting script."
        break
    fi
    echo ""
done