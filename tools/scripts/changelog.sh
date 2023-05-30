#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# usage:
#  GH_TOKEN=${{ github.token }} LABEL_FEATURE="release/feature-new" LABEL_BUG="release/bug" PROJECT_REPO="spidernet-io/spiderpool" changelog.sh ./ v0.3.6
#  GH_TOKEN=${{ github.token }} LABEL_FEATURE="release/feature-new" LABEL_BUG="release/bug" PROJECT_REPO="spidernet-io/spiderpool" changelog.sh ./ v0.3.6 v0.3.5


CURRENT_DIR_PATH=$(cd `dirname $0`; pwd)
PROJECT_ROOT_PATH="${CURRENT_DIR_PATH}/../.."

set -x
set -o errexit
set -o nounset
set -o pipefail

#required
OUTPUT_DIR=${1}
#required
DEST_TAG=${2}
# optional
START_TAG=${3:-""}

LABEL_FEATURE=${LABEL_FEATURE:-"release/feature-new"}
LABEL_BUG=${LABEL_BUG:-"release/bug"}
PROJECT_REPO=${PROJECT_REPO:-"spidernet-io/spiderpool"}

OUTPUT_DIR=$(cd ${OUTPUT_DIR}; pwd)
echo "generate changelog to directory ${OUTPUT_DIR}"
cd ${OUTPUT_DIR}

ORIGIN_START_TAG=${START_TAG}
if [ -z "${START_TAG}" ] ; then
    echo "-------------- generate start tag"
    VERSION=` grep -oE "[0-9]+\.[0-9]+\.[0-9]+" <<< "${DEST_TAG}" `
    V_X=${VERSION%%.*}
    TMP=${VERSION%.*}
    V_Y=${TMP#*.}
    V_Z=${VERSION##*.}
    RC=` sed -E 's?[vV]*[0-9]+\.[0-9]+\.[0-9]+[^0-9]*??' <<<  "${DEST_TAG}" `
    #---------
    START_X=""
    START_Y=""
    START_Z=""
    START_RC=""
    #--------
    SET_VERSION() {
      if (( V_Z == 0 )); then
        if (( V_Y == 0 )); then
          if (( V_X > 0 )); then
            START_X=$(( V_X - 1 ))
            # ??
            START_Y=0
            START_Z=0
          else
            echo "error, $DEST_TAG, all 0"
            exit 0
          fi
        else
          START_X=$V_X
          START_Y=$(( V_Y - 1 ))
          START_Z=0
        fi
      else
        START_X=$V_X
        START_Y=$V_Y
        START_Z=$(( V_Z - 1 ))
      fi
    }

    TMP_DEST_TAG=$DEST_TAG
    if [ -z "${RC}" ] ;then
      SET_VERSION
    else
      if (( RC == 0 )) ;then
        SET_VERSION
        # remove rc
        TMP_DEST_TAG=` grep -oE "[vV]*[0-9]+\.[0-9]+\.[0-9]+" <<< "${DEST_TAG}" `
        RC=""
      else
        START_X=$V_X
        START_Y=$V_Y
        START_Z=$V_Z
        START_RC=$(( RC - 1 ))
      fi
    fi
    #------ result
    START_TAG=` sed -E "s?[0-9]+\.[0-9]+\.[0-9]+?${START_X}.${START_Y}.${START_Z}?" <<<  "${TMP_DEST_TAG}" `
    if [ -n "${RC}" ] ; then
      START_TAG=` sed -E "s?([vV]*[0-9]+\.[0-9]+\.[0-9]+[^0-9]*[^0-9]*)[0-9]+?\1${START_RC}?" <<<  "${START_TAG}" `
    fi
fi

echo "-------------- check tags "
echo "DEST_TAG=${DEST_TAG}"
echo "START_TAG=${START_TAG}"

# check whether tag START_TAG  exists
ALL_PR_INFO=""
if [ -z "${ORIGIN_START_TAG}" ] && (( START_X == 0 )) && (( START_Y == 0 )) && (( START_Z == 0 )); then
	ALL_PR_INFO=`git log ${DEST_TAG} --reverse --merges --oneline` \
		|| { echo "error, failed to get PR for tag ${DEST_TAG} " ; exit 1 ; }
else
	ALL_PR_INFO=`git log ${START_TAG}..${DEST_TAG} --reverse --merges --oneline` \
		|| { echo "error, failed to get PR for tag ${DEST_TAG} " ; exit 1 ; }
fi

ALL_PR_NUM=` grep -oE " Merge pull request #[0-9]+ " <<< "$ALL_PR_INFO" | grep -oE "[0-9]+" `
TOTAL_COUNT=` wc -l <<< "${ALL_PR_NUM}" `

#
FEATURE_PR=""
FIX_PR=""
for PR in ${ALL_PR_NUM} ; do
	INFO=` gh pr view ${PR}  `
	TITLE=` grep -E "^title:[[:space:]]+" <<< "$INFO" | sed -E 's/title:[[:space:]]+//' `
	LABELS=` grep -E "^labels:[[:space:]][^\[]" <<< "$INFO" | sed -E 's/labels://' | tr ',' ' ' `
	#
	PR_URL="https://github.com/${PROJECT_REPO}/pull/${PR}"
	#
	if grep -E "[[:space:]]${LABEL_FEATURE}[[:space:]]" <<< " ${LABELS} " &>/dev/null ; then
		FEATURE_PR+="* ${TITLE} : [PR ${PR}](${PR_URL})
"
	elif grep -E "[[:space:]]${LABEL_BUG}[[:space:]]" <<< " ${LABELS} " &>/dev/null ; then
		FIX_PR+="* ${TITLE} : [PR ${PR}](${PR_URL})
"
	fi
done
#---------------------
echo "generate changelog md"
FILE_CHANGELOG="${OUTPUT_DIR}/changelog_from_${START_TAG}_to_${DEST_TAG}.md"
echo > ${FILE_CHANGELOG}
echo "# ${DEST_TAG}" >> ${FILE_CHANGELOG}
echo "" >> ${FILE_CHANGELOG}
echo "***" >> ${FILE_CHANGELOG}
echo "" >> ${FILE_CHANGELOG}
#
if [ -n "${FEATURE_PR}" ]; then
    echo "## Feature" >> ${FILE_CHANGELOG}
    echo "" >> ${FILE_CHANGELOG}
    while read LINE ; do
      echo "${LINE}" >> ${FILE_CHANGELOG}
      echo "" >> ${FILE_CHANGELOG}
    done <<< "${FEATURE_PR}"
    echo "***" >> ${FILE_CHANGELOG}
    echo "" >> ${FILE_CHANGELOG}
fi
#
if [ -n "${FIX_PR}" ]; then
    echo "## Fix" >> ${FILE_CHANGELOG}
    echo "" >> ${FILE_CHANGELOG}
    while read LINE ; do
      echo "${LINE}" >> ${FILE_CHANGELOG}
      echo "" >> ${FILE_CHANGELOG}
    done <<< "${FIX_PR}"
    echo "***" >> ${FILE_CHANGELOG}
    echo "" >> ${FILE_CHANGELOG}
fi
#
echo "## Totoal PR" >> ${FILE_CHANGELOG}
echo "" >> ${FILE_CHANGELOG}
echo "[ ${TOTAL_COUNT} PR](https://github.com/${PROJECT_REPO}/compare/${START_TAG}...${DEST_TAG})" >> ${FILE_CHANGELOG}
echo "--------------------"
echo "generate changelog to ${FILE_CHANGELOG}"
