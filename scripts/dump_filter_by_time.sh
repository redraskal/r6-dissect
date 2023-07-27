#!/bin/bash

if [ $# -ne 2 ]; then
    echo "Usage: $0 <input_file> <time_string>"
    exit 1
fi

input_file="$1"
time_string="$2"

if [ ! -f "$input_file" ]; then
    echo "Error: Input file not found."
    exit 1
fi

output_file="${time_string//:/_}.txt"

start_line=$(grep -n "$time_string" "$input_file" | cut -d ':' -f 1)

if [ -n "$start_line" ]; then
    echo "$time_string:" > "$output_file"
    ((start_line++))
    sed -n "${start_line},/---------------/p" "$input_file" >> "$output_file"
    echo "Data saved to $output_file"
else
    echo "Pattern not found in the input file."
fi
