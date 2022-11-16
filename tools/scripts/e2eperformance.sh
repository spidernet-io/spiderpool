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
DATA1=$( LoopList::GetElement "$E2E_DATA" "SuiteDescription" '"Performance Suite"' )
DATA2=$( LoopList::GetElement "$E2E_DATA" "SuiteDescription" '"Subnet Suite"' )
(($?!=0)) && echo "error, failed to get SuiteDescription " >&2  && echo "$E2E_DATA" >&2 && exit 1

E2E_DATA1=$( jq '.SpecReports' <<< "$DATA1" )
E2E_DATA2=$( jq '.SpecReports' <<< "$DATA2" )
DATA1=$( LoopList::GetElement "$E2E_DATA1" "LeafNodeLabels"  '["P00002"]'  )
DATA2=$( LoopList::GetElement "$E2E_DATA2" "LeafNodeLabels"  '["I00006","I00008"]'  )
(($?!=0)) && echo "error, failed to get LeafNodeLabels" >&2  && echo "$E2E_DATA1" >&2 && echo "$E2E_DATA2" >&2 && exit 1

CASE_STATE1=$( jq '.State' <<< "$DATA1" | tr -d '"' )
CASE_STATE2=$( jq '.State' <<< "$DATA2" | tr -d '"' )
(($?!=0)) && echo "error, failed to get case status " >&2  && echo "$DATA1" >&2 && echo "$DATA2" >&2 && exit 1
if [ "$CASE_STATE1" != "passed" ] && [ "$CASE_STATE2" != "passed" ] ; then
    echo "the state of performance test case P00002 && I00006 is $CASE_STATE1 or $CASE_STATE2, ignore retrieve result" >&2
    exit 0
fi

if [ "$CASE_STATE1" == "passed" ] ; then
    E2E_DATA1=$( jq '.ReportEntries' <<< "$DATA1" )
    DATA1=$( LoopList::GetElement "$E2E_DATA1" "Name"  '"Performance Results"'  )
    (($?!=0)) && echo "error, failed to get ReportEntries" >&2 && echo "$E2E_DATA1" >&2 && exit 1

    ENTRY_DATA1=$( jq '.Value.Representation' <<< "$DATA1" | sed 's/\\//g')
    (($?!=0)) && echo "error, failed to get Representation" >&2 && echo "$ENTRY_DATA1" >&2 && exit 1

    # "{ "controllerType" : "deployment", "replicas": 60, "createTime": 60 , "rebuildTime": 100, "deleteTime": 48 }"
    # to
    # { "controllerType" : "deployment", "replicas": 60, "createTime": 60 , "rebuildTime": 100, "deleteTime": 48 }
    ENTRY_DATA1=$( echo "$ENTRY_DATA1" | sed 's/"{/{/' | sed 's/}"/}/' )
    (($?!=0)) && echo "error, failed to get data" >&2  && echo "$ENTRY_DATA1" >&2 && exit 1

    createTime=$( jq '.createTime' <<< "$ENTRY_DATA1" )
    rebuildTime=$( jq '.rebuildTime' <<< "$ENTRY_DATA1" )
    deleteTime=$( jq '.deleteTime' <<< "$ENTRY_DATA1" )

    [ -z "$createTime" ] && echo "error, failed to get createTime " >&2  && echo "$ENTRY_DATA1" >&2 && exit 1
    [ -z "$rebuildTime" ] && echo "error, failed to get rebuildTime " >&2  && echo "$ENTRY_DATA1" >&2 && exit 1
    [ -z "$deleteTime" ] && echo "error, failed to get deleteTime " >&2  && echo "$ENTRY_DATA1" >&2 && exit 1
fi

if [ "$CASE_STATE2" == "passed" ] ; then

    E2E_DATA2=$( jq '.ReportEntries' <<< "$DATA2" )
    DATA2=$( LoopList::GetElement "$E2E_DATA2" "Name"  '"Subnet Performance Results"'  )
    (($?!=0)) && echo "error, failed to get ReportEntries" >&2 && echo "$E2E_DATA2" >&2 && exit 1

    ENTRY_DATA2=$( jq '.Value.Representation' <<< "$DATA2" | sed 's/\\//g')
    (($?!=0)) && echo "error, failed to get Representation" >&2 &&  echo "$ENTRY_DATA2" >&2 && exit 1

    ENTRY_DATA2=$( echo "$ENTRY_DATA2" | sed 's/"{/{/' | sed 's/}"/}/' )
    (($?!=0)) && echo "error, failed to get data" >&2  && echo "$ENTRY_DATA2" >&2 && exit 1

    subnetCreateTime=$( jq '.createTime' <<< "$ENTRY_DATA2" )
    subnetScaleUpAndDownTime=$( jq '.scaleupAndScaledownTime' <<< "$ENTRY_DATA2" )
    subnetDeleteTime=$( jq '.deleteTime' <<< "$ENTRY_DATA2" )

    [ -z "$subnetCreateTime" ] && echo "error, failed to get createTime " >&2  && echo "$ENTRY_DATA2" >&2 && exit 1
    [ -z "$subnetScaleUpAndDownTime" ] && echo "error, failed to get scale up and down time " >&2  && echo "$ENTRY_DATA2" >&2 && exit 1
    [ -z "$subnetDeleteTime" ] && echo "error, failed to get deleteTime " >&2  && echo "$ENTRY_DATA2" >&2 && exit 1
fi

if [ "$CASE_STATE1" == "passed" ] && [ "$CASE_STATE2" == "passed" ] ; then
    echo "${createTime}/${rebuildTime}/${deleteTime} | ${subnetCreateTime}/${subnetScaleUpAndDownTime}/${subnetDeleteTime}"
fi

if [ "$CASE_STATE1" == "passed" ] && [ "$CASE_STATE2" != "passed" ] ; then
    echo "${createTime}/${rebuildTime}/${deleteTime}"
fi

if [ "$CASE_STATE1" != "passed" ] && [ "$CASE_STATE2" == "passed" ] ; then
    echo "${subnetCreateTime}/${subnetScaleUpAndDownTime}/${subnetDeleteTime}"
fi