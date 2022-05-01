#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

SCRIPT_DIR="$( cd "$( dirname "$0" )" && pwd )"
PROJECT_DIR="${SCRIPT_DIR}/../.."
VEDNOR_DIR="vendor"

SUMMARY="$(cloc "${PROJECT_DIR}"  --exclude-dir="${VEDNOR_DIR}" --md | tail -1)"
IFS='|' read -r -a TOKENS <<< "$SUMMARY"

NUMBER_OF_FILES=${TOKENS[1]}
COMMENT_LINES=${TOKENS[3]}
LINES_OF_CODE=${TOKENS[4]}

DUMB_COMMENTS="$(grep -r -E '//////|// -----' "${PROJECT_DIR}" | wc -l)"
COMMENT_LINES=$(($COMMENT_LINES - 5 * $NUMBER_OF_FILES - $DUMB_COMMENTS))

if [[ $# -eq 0 ]] ; then
  awk -v a=$LINES_OF_CODE \
      'BEGIN {printf "Lines of source code: %6.1fk\n", a/1000}'
  awk -v a=$COMMENT_LINES \
      'BEGIN {printf "Lines of comments:    %6.1fk\n", a/1000}'
  awk -v a=$COMMENT_LINES -v b=$LINES_OF_CODE \
      'BEGIN {printf "Comment Percentage:   %6.1f\n", 100*a/(a+b)}'
  exit 0
fi

if [[ $* == *--code-lines* ]] ; then
  awk -v a=$LINES_OF_CODE \
      'BEGIN {printf "%.1fk\n", a/1000}'
fi

if [[ $* == *--comment-lines* ]] ; then
  awk -v a=$COMMENT_LINES \
      'BEGIN {printf "%.1fk\n", a/1000}'
fi

if [[ $* == *--comment-percent* ]] ; then
  awk -v a=$COMMENT_LINES -v b=$LINES_OF_CODE \
      'BEGIN {printf "%.1f\n", 100*a/(a+b)}'
fi
