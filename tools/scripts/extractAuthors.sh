#!/usr/bin/env bash

# Copyright 2024 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -x
set -o nounset
set -o pipefail

# Ensure sort order doesn't depend on locale
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8

function extract_authors() {
    extract_email=$(git log --use-mailmap --format="%<|(40)%aN%aE" --date=iso-local \
        | awk '!seen[$(NF-1)]++' \
        | awk '!seen[$NF]++' \
        | grep -v noreply.github.com \
        | grep -v example.com \
        | awk '{print $NF}')
        
    # Iterate $extract_email by line
	IFS=$'\n'
    for i in $extract_email; do
        if [[ ! $i =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
            echo "Invalid email format: $i" &>/dev/null
            continue
        fi
        git log --use-mailmap --format="%<|(40)%aN%aE" \
        | grep -v noreply.github.com \
        | grep -v example.com \
        | grep $i \
        | sort \
        | head -n 1
	done
}

extract_authors | sort -u