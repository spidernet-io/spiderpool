#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

CURRENT_DIR_PATH=$( dirname $0 )
CURRENT_DIR_PATH=$(cd ${CURRENT_DIR_PATH} ; pwd)
PROJECT_ROOT_PATH=$(cd ${CURRENT_DIR_PATH}/../.. ; pwd)

E2E_REPORT_PATH="$1"
if [ ! -f "$E2E_REPORT_PATH" ] ; then
    echo "error, no file $E2E_REPORT_PATH " >&2
    exit 1
fi

LoopList::GetElement() {
    LIST_DATA="$1"
    EXPECTED_KEY="$2"
    EXPECTED_VALUE="$3"
    DEBUG="$4"

    LENGTH=$( jq '. | length' <<< "$LIST_DATA"  )
    #echo "map have $LENGTH section" >&2
    for(( NUM=0 ; NUM <LENGTH ; NUM++ )) ; do
        VALUE=$( jq -c '.['"$NUM"'].'"${EXPECTED_KEY}"''  <<< "$LIST_DATA" 2>/dev/null )
        if [ -n "$VALUE" ]; then
            [ -n "$DEBUG" ] && echo "$VALUE" >&2
            if [ -n "$EXPECTED_VALUE" ] ; then
                [ "$VALUE" == "$EXPECTED_VALUE" ] && jq '.['"$NUM"']'  <<< "$LIST_DATA" 2>/dev/null && return 0
            else
                jq '.['"$NUM"']'  <<< "$LIST_DATA" 2>/dev/null && return 0
            fi
        fi
    done
    return 1
}


E2E_DATA=$( cat "$E2E_REPORT_PATH" )
DATA=$( LoopList::GetElement "$E2E_DATA" "SuiteDescription" '"Performance Suite"' )
(($?!=0)) && echo "error, failed to get SuiteDescription " >&2  && echo "$E2E_DATA" >&2 && exit 1

E2E_DATA=$( jq '.SpecReports' <<< "$DATA" )
DATA=$( LoopList::GetElement "$E2E_DATA" "LeafNodeLabels"  '["P00002"]'  )
(($?!=0)) && echo "error, failed to get LeafNodeLabels" >&2  && echo "$E2E_DATA" >&2 && exit 1


CASE_STATE=$( jq '.State' <<< "$DATA" | tr -d '"' )
(($?!=0)) && echo "error, failed to get case status " >&2  && echo "$DATA" >&2 && exit 1
if [ "$CASE_STATE" != "passed" ] ; then
    echo "the state of performance test case P00002 is $CASE_STATE, ignore retrieve result" >&2
    exit 0
fi

E2E_DATA=$( jq '.ReportEntries' <<< "$DATA" )
DATA=$( LoopList::GetElement "$E2E_DATA" "Name"  '"Multi-ippool Performance Results"'  )
(($?!=0)) && echo "error, failed to get ReportEntries" >&2 && echo "$E2E_DATA" >&2 && exit 1

ENTRY_DATA=$( jq '.Value.Representation' <<< "$DATA" | sed 's/\\//g')
(($?!=0)) && echo "error, failed to get Representation" >&2 && echo "$ENTRY_DATA" >&2 && exit 1

ENTRY_DATA=$( echo "$ENTRY_DATA" | sed 's/"{/{/' | sed 's/}"/}/' )
(($?!=0)) && echo "error, failed to get data" >&2  && echo "$ENTRY_DATA" >&2 && exit 1

createTime=$( jq '.createTime' <<< "$ENTRY_DATA" )
rebuildTime=$( jq '.rebuildTime' <<< "$ENTRY_DATA" )
deleteTime=$( jq '.deleteTime' <<< "$ENTRY_DATA" )

[ -z "$createTime" ] && echo "error, failed to get createTime " >&2  && echo "$ENTRY_DATA" >&2 && exit 1
[ -z "$rebuildTime" ] && echo "error, failed to get rebuildTime " >&2  && echo "$ENTRY_DATA" >&2 && exit 1
[ -z "$deleteTime" ] && echo "error, failed to get deleteTime " >&2  && echo "$ENTRY_DATA" >&2 && exit 1

echo "${createTime}/${rebuildTime}/${deleteTime}"