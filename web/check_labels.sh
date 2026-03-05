#!/bin/bash

check_input_labels() {
  local file="$1"
  # Check for inputs without aria-label and without id (indicating no htmlFor label)
  grep -n "<input" "$file" | while read -r line; do
    line_num=$(echo "$line" | cut -d: -f1)
    # Get context around the line to check for labels
    context_start=$((line_num - 3))
    context_end=$((line_num + 3))
    
    # Extract the input tag (may span multiple lines)
    input_line=$(sed -n "${line_num},$((line_num + 10))p" "$file" | tr '\n' ' ')
    
    # Check if has aria-label
    if echo "$input_line" | grep -q "aria-label"; then
      continue
    fi
    
    # Check if has id
    if echo "$input_line" | grep -q 'id="'; then
      continue
    fi
    
    # This is potentially unlabeled - check previous context for label
    context=$(sed -n "${context_start},${line_num}p" "$file")
    if ! echo "$context" | grep -q "htmlFor\|aria-label"; then
      echo "FILE: $file"
      echo "LINE: $line_num"
      sed -n "${context_start},${context_end}p" "$file"
      echo "---"
    fi
  done
}

for file in app/**/*.tsx components/**/*.tsx; do
  check_input_labels "$file" 2>/dev/null
done
