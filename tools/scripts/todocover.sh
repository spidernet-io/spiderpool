#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0


CURRENT_FILENAME=$( basename $0 )
CURRENT_DIR_PATH=$(cd $(dirname $0); pwd)
PROJECT_ROOT=$( cd ${CURRENT_DIR_PATH}/../.. && pwd )

NUM=$( grep -E -i  "//[[:space:]]*TODO[[:space:]]*\(.*\)" ${PROJECT_ROOT} -R  --exclude-dir=vendor --exclude-dir=.git | wc -l )

echo $NUM
