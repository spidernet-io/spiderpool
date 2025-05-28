#!/bin/bash
# Copyright 2025 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

# This script translates a markdown changelog file from English to Chinese
# while preserving all markdown formatting

# Check for required parameters
if [ -z "${1:-}" ] || [ -z "${2:-}" ]; then
  echo "Usage: $0 <input_file> <output_file>"
  echo "Example: $0 ./changelog-en.md ./changelog-zh.md"
  exit 1
fi

INPUT_FILE="$1"
OUTPUT_FILE="$2"

# Check if input file exists
if [ ! -f "$INPUT_FILE" ]; then
  echo "Error: Input file '$INPUT_FILE' does not exist"
  exit 1
fi

# Ensure translate-markdown is installed
if ! command -v translate-markdown &> /dev/null; then
  echo "Installing translate-markdown..."
  pip3 install translate-markdown
fi

echo "Translating markdown file from English to Chinese..."
echo "Input: $INPUT_FILE"
echo "Output: $OUTPUT_FILE"

# Translate the markdown file while preserving format, retry 3 times
success=false
for i in $(seq 3); do
  echo "Translation attempt $i of 3..."
  if translate-markdown -i "$INPUT_FILE" -o "$OUTPUT_FILE" -l zh; then
    # Check if translation was successful and file is not empty
    if [ -s "$OUTPUT_FILE" ]; then
      echo "Translation successful on attempt $i"
      success=true
      break
    fi
  fi
  echo "Translation attempt $i failed, retrying..."
  sleep 2
done

# If all translation attempts failed, exit with error
if [ "$success" != "true" ]; then
  echo "All translation attempts failed after 3 retries"
  exit 1
fi

echo "Successfully translated markdown file with format preserved"
