#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

# usage:
#  GH_TOKEN=${{ github.token }} LABEL_FEATURE="release/feature-new" LABEL_CHANGED="release/feature-changed" LABEL_BUG="release/bug" PROJECT_REPO="xxx/xxx" changelog.sh  ./ v0.3.6
#  GH_TOKEN=${{ github.token }} LABEL_FEATURE="release/feature-new" LABEL_CHANGED="release/feature-changed" LABEL_BUG="release/bug" PROJECT_REPO="xxx/xxx" changelog.sh  ./ v0.3.6 v0.3.5

CURRENT_DIR_PATH=$(
  cd $(dirname $0)
  pwd
)
PROJECT_ROOT_PATH="${CURRENT_DIR_PATH}/../.."

set -x
set -o errexit
set -o nounset
set -o pipefail

# optional
OUTPUT_DIR=${1}
# required
DEST_TAG=${2}
# optional
START_TAG=${3:-""}

LABEL_FEATURE=${LABEL_FEATURE:-"release/feature-new"}
LABEL_CHANGED=${LABEL_CHANGED:-"release/feature-changed"}
LABEL_BUG=${LABEL_BUG:-"release/bug"}
LABEL_NONE=${LABEL_NONE:-"release/none"}
PROJECT_REPO=${PROJECT_REPO:-""}
[ -n "$PROJECT_REPO" ] || {
  echo "miss PROJECT_REPO"
  exit 1
}
[ -n "${GH_TOKEN}" ] || {
  echo "error, miss GH_TOKEN"
  exit 1
}

(cd ${OUTPUT_DIR}) || {
  echo "error, OUTPUT_DIR '${OUTPUT_DIR}' is not a valid directory"
  exit 1
}
OUTPUT_DIR=$(
  cd ${OUTPUT_DIR}
  pwd
)
echo "generate changelog to directory ${OUTPUT_DIR}"

# change to root for git cli
cd ${PROJECT_ROOT_PATH}

#============================
echo "-------------- generate latest release version tag --------------"
LATEST_RELEASE_VERISON=$(curl --retry 10 -H "Accept: application/vnd.github+json" -H "Authorization: Bearer ${GH_TOKEN}" -s https://api.github.com/repos/spidernet-io/spiderpool/releases | grep '"tag_name":' | grep -Eo "v([0-9]+\.[0-9]+\.[0-9])" | sort -r | head -n 1)
LATEST_RELEASE_VERISON=$(grep -oE "[0-9]+\.[0-9]+\.[0-9]+" <<<"${LATEST_RELEASE_VERISON}")
if [ -z "${LATEST_RELEASE_VERISON}" ]; then
  LATEST_X=0
  LATEST_Y=0
  LATEST_Z=0
else
  LATEST_X=${LATEST_RELEASE_VERISON%%.*}
  TMP=${LATEST_RELEASE_VERISON%.*}
  LATEST_Y=${TMP#*.}
  LATEST_Z=${LATEST_RELEASE_VERISON##*.}
fi

extract_release_notes() {
  local PR_INFO=$1
  local RELEASE_NOTE=""

  [ -z "${PR_INFO}" ] && {
    echo "error, PR_INFO is empty"
    return
  }

  # use awk to extract release note from PR_INFO
  RELEASE_NOTE=$(echo "$PR_INFO" | jq -r .body | awk '/```release-note/{flag=1;next}/```/{if(flag==1){flag=0;exit}}flag' | sed 's/^[[:space:]]*//')

  # if above method failed, try another method
  if [ -z "$RELEASE_NOTE" ]; then
    local BODY=$(echo "$PR_INFO" | jq -r .body)
    local START_LINE=$(echo "$BODY" | grep -n '```release-note' | cut -d ':' -f1)
    if [ -n "$START_LINE" ]; then
      local END_LINE=$(echo "$BODY" | tail -n +$((START_LINE + 1)) | grep -n '```' | head -n 1 | cut -d ':' -f1)
      if [ -n "$END_LINE" ]; then
        RELEASE_NOTE=$(echo "$BODY" | tail -n +$((START_LINE + 1)) | head -n $((END_LINE - 1)) | sed 's/^[[:space:]]*//')
      fi
    fi
  fi

  echo -e "$RELEASE_NOTE"
}

#============================
ORIGIN_START_TAG=${START_TAG}
if [ -z "${START_TAG}" ]; then
  echo "-------------- generate start tag"
  VERSION=$(grep -oE "[0-9]+\.[0-9]+\.[0-9]+" <<<"${DEST_TAG}")
  V_X=${VERSION%%.*}
  TMP=${VERSION%.*}
  V_Y=${TMP#*.}
  V_Z=${VERSION##*.}
  RC=$(sed -E 's?[vV]*[0-9]+\.[0-9]+\.[0-9]+[^0-9]*??' <<<"${DEST_TAG}")
  #---------
  START_X=""
  START_Y=""
  START_Z=""
  START_RC=""
  #--------
  SET_VERSION() {
    if ((V_Z == 0)); then
      if ((V_Y == 0)); then
        if ((V_X > 0)); then
          START_X=$((V_X - 1))
          # ??
          START_Y=$LATEST_Y
          START_Z=$LATEST_Z
        else
          echo "error, $DEST_TAG, all 0"
          exit 0
        fi
      else
        START_X=$V_X
        START_Y=$((V_Y - 1))
        START_Z=0
      fi
    else
      START_X=$V_X
      START_Y=$V_Y
      START_Z=$((V_Z - 1))
    fi
  }

  TMP_DEST_TAG=$DEST_TAG
  if [ -z "${RC}" ]; then
    SET_VERSION
  else
    if ((RC == 0)); then
      SET_VERSION
      # remove rc
      TMP_DEST_TAG=$(grep -oE "[vV]*[0-9]+\.[0-9]+\.[0-9]+" <<<"${DEST_TAG}")
      RC=""
    else
      START_X=$V_X
      START_Y=$V_Y
      START_Z=$V_Z
      START_RC=$((RC - 1))
    fi
  fi
  #------ result
  START_TAG=$(sed -E "s?[0-9]+\.[0-9]+\.[0-9]+?${START_X}.${START_Y}.${START_Z}?" <<<"${TMP_DEST_TAG}")
  if [ -n "${RC}" ]; then
    START_TAG=$(sed -E "s?([vV]*[0-9]+\.[0-9]+\.[0-9]+[^0-9]*[^0-9]*)[0-9]+?\1${START_RC}?" <<<"${START_TAG}")
  fi
fi

echo "-------------- check tags "
echo "DEST_TAG=${DEST_TAG}"
echo "START_TAG=${START_TAG}"

# check whether tag START_TAG  exists
ALL_COMMIT=""
if [ -z "${ORIGIN_START_TAG}" ] && ((START_X == 0)) && ((START_Y == 0)) && ((START_Z == 0)); then
  ALL_COMMIT=$(git log ${DEST_TAG} --reverse --oneline | awk '{print $1}' | tr '\n' ' ') ||
    {
      echo "error, failed to get PR for tag ${DEST_TAG} "
      exit 1
    }
else
  ALL_COMMIT=$(git log ${START_TAG}..${DEST_TAG} --reverse --oneline | awk '{print $1}' | tr '\n' ' ') ||
    {
      echo "error, failed to get PR for tag ${DEST_TAG} "
      exit 1
    }
fi

if [ -z "${ALL_COMMIT}" ]; then
  echo "error, failed to get PR for tag ${DEST_TAG} "
  exit 1
fi

echo "ALL_COMMIT: ${ALL_COMMIT}"
rm -f test.log
touch test.log

TOTAL_COUNT="0"
PR_LIST=""
#
FEATURE_PR_EN=""
CHANGED_PR_EN=""
FIX_PR_EN=""
FEATURE_PR_ZH=""
CHANGED_PR_ZH=""
FIX_PR_ZH=""
for COMMIT in ${ALL_COMMIT}; do
  # API RATE LIMIT
  # https://docs.github.com/en/rest/overview/resources-in-the-rest-api?apiVersion=2022-11-28#rate-limiting
  # When using GITHUB_TOKEN, the rate limit is 1,000 requests per hour per repository
  # GitHub Enterprise Cloud's rate limit applies, and the limit is 15,000 requests per hour per repository.
  COMMIT_INFO=$(curl --retry 10 -s -H "Accept: application/vnd.github.groot-preview+json" -H "Authorization: Bearer ${GH_TOKEN}" https://api.github.com/repos/${PROJECT_REPO}/commits/${COMMIT}/pulls)
  PR=$(jq -r '.[].number' <<<"${COMMIT_INFO}")
  [ -n "${PR}" ] || {
    echo "warning, failed to find PR number for commit ${COMMIT} "
    echo "${COMMIT_INFO}"
    echo ""
    continue
  }
  if grep " ${PR} " <<<" ${PR_LIST} " &>/dev/null; then
    continue
  else
    PR_LIST+=" ${PR} "
  fi
  ((TOTAL_COUNT += 1))
  PR_INFO=$(curl -s -H "Accept: application/vnd.github.groot-preview+json" -H "Authorization: Bearer ${GH_TOKEN}" "https://api.github.com/repos/$PROJECT_REPO/pulls/$PR")
  LABELS=$(jq -r '[.labels[].name] | join(", ")' <<<"$PR_INFO")
  PR_USER=$(jq -r .user.login <<<"$PR_INFO")

  # release/feature-new, cherrypick-release-v0.8, cherrypick-release-v0.9, cherrypick-release-v1.0
  # Skip PR if it doesn't have any release labels
  if ! echo "${LABELS}" | grep -E "(${LABEL_FEATURE}|${LABEL_CHANGED}|${LABEL_BUG})" &>/dev/null; then
    echo "PR ${PR} has no release labels, ignore"
    continue
  fi

  # get release notes
  RELEASE_NOTE_CONTENT=$(extract_release_notes "$PR_INFO")

  if [ -z "$RELEASE_NOTE_CONTENT" ]; then
    echo "warning, failed to get release notes for PR ${PR}"
    echo "${PR_INFO}"
    echo ""
    continue
  fi

  # extract en release note (with "en:" prefix)
  EN_RELEASE_NOTE=$(echo "$RELEASE_NOTE_CONTENT" | grep -E "^en:" | sed 's/^en://')

  # extract zh release note (with "zh:" prefix)
  ZH_RELEASE_NOTE=$(echo "$RELEASE_NOTE_CONTENT" | grep -E "^zh:" | sed 's/^zh://')

  # PR_USER_URL=$(echo $PR_INFO | jq -r .user.html_url)
  #
  PR_URL="https://github.com/${PROJECT_REPO}/pull/${PR}"
  #
  if echo " ${LABELS} " | grep -E "${LABEL_FEATURE}" &>/dev/null; then
    echo "get new feature PR ${PR}"
    # use release notes to enrich changelog
    if [ -n "$EN_RELEASE_NOTE" ]; then
      FEATURE_PR_EN+="* ${EN_RELEASE_NOTE}: [PR ${PR}](${PR_URL}) (@${PR_USER})
"
    fi

    if [ -n "$ZH_RELEASE_NOTE" ]; then
      FEATURE_PR_ZH+="* ${ZH_RELEASE_NOTE}: [PR ${PR}](${PR_URL})
"
    fi
  elif echo " ${LABELS} " | grep -E "${LABEL_CHANGED}" &>/dev/null; then
    echo "get changed feature PR ${PR}"
    if [ -n "$EN_RELEASE_NOTE" ]; then
      CHANGED_PR_EN+="* ${EN_RELEASE_NOTE}: [PR ${PR}](${PR_URL})(@${PR_USER})
"
    fi
    if [ -n "$ZH_RELEASE_NOTE" ]; then
      CHANGED_PR_ZH+="* ${ZH_RELEASE_NOTE}: [PR ${PR}](${PR_URL})
"
    fi
  elif echo " ${LABELS} " | grep -E "${LABEL_BUG}" &>/dev/null; then
    echo "get bug fix PR ${PR}"
    if [ -n "$EN_RELEASE_NOTE" ]; then
      FIX_PR_EN+="* ${EN_RELEASE_NOTE}: [PR ${PR}](${PR_URL})(@${PR_USER})
"
    fi
    if [ -n "$ZH_RELEASE_NOTE" ]; then
      FIX_PR_ZH+="* ${ZH_RELEASE_NOTE}: [PR ${PR}](${PR_URL})
"
    fi
  fi
done
#---------------------
echo "generate changelog md"
FILE_CHANGELOG="${OUTPUT_DIR}/changelog_from_${START_TAG}_to_${DEST_TAG}.md"
ZH_CN_FILE_CHANGELOG="${OUTPUT_DIR}/changelog_from_${START_TAG}_to_${DEST_TAG}_zh-cn.md"
echo >${FILE_CHANGELOG}
echo >${ZH_CN_FILE_CHANGELOG}
echo "# ${DEST_TAG}" >>${FILE_CHANGELOG}
echo "# ${DEST_TAG}" >>${ZH_CN_FILE_CHANGELOG}
echo "Welcome to the ${DEST_TAG} release of Spiderpool!" >>${FILE_CHANGELOG}
echo "Compared with version:${START_TAG}, version:${DEST_TAG} has the following updates." >>${FILE_CHANGELOG}
echo "" >>${FILE_CHANGELOG}
echo "***" >>${FILE_CHANGELOG}
echo "" >>${FILE_CHANGELOG}
echo "" >>${ZH_CN_FILE_CHANGELOG}
#
if [ -n "${FEATURE_PR_EN}" ]; then
  echo "## New Feature" >>${FILE_CHANGELOG}
  echo "" >>${FILE_CHANGELOG}
  while read LINE; do
    echo "${LINE}" >>${FILE_CHANGELOG}
    echo "" >>${FILE_CHANGELOG}
  done <<<"${FEATURE_PR_EN}"
  echo "***" >>${FILE_CHANGELOG}
  echo "" >>${FILE_CHANGELOG}
fi

if [ -n "${FEATURE_PR_ZH}" ]; then
  echo "## 新功能" >>${ZH_CN_FILE_CHANGELOG}
  echo "" >>${ZH_CN_FILE_CHANGELOG}
  while read LINE; do
    echo "${LINE}" >>${ZH_CN_FILE_CHANGELOG}
    echo "" >>${ZH_CN_FILE_CHANGELOG}
  done <<<"${FEATURE_PR_ZH}"
  echo "***" >>${ZH_CN_FILE_CHANGELOG}
  echo "" >>${ZH_CN_FILE_CHANGELOG}
fi

#
if [ -n "${CHANGED_PR_EN}" ]; then
  echo "## Changed Feature" >>${FILE_CHANGELOG}
  echo "" >>${FILE_CHANGELOG}
  while read LINE; do
    echo "${LINE}" >>${FILE_CHANGELOG}
    echo "" >>${FILE_CHANGELOG}
  done <<<"${CHANGED_PR_EN}"
  echo "***" >>${FILE_CHANGELOG}
  echo "" >>${FILE_CHANGELOG}
fi

if [ -n "${CHANGED_PR_ZH}" ]; then
  echo "## 功能更新" >>${ZH_CN_FILE_CHANGELOG}
  echo "" >>${ZH_CN_FILE_CHANGELOG}
  while read LINE; do
    echo "${LINE}" >>${ZH_CN_FILE_CHANGELOG}
    echo "" >>${ZH_CN_FILE_CHANGELOG}
  done <<<"${CHANGED_PR_ZH}"
  echo "***" >>${ZH_CN_FILE_CHANGELOG}
  echo "" >>${ZH_CN_FILE_CHANGELOG}
fi
#
if [ -n "${FIX_PR_EN}" ]; then
  echo "## Fixes" >>${FILE_CHANGELOG}
  echo "" >>${FILE_CHANGELOG}
  while read LINE; do
    echo "${LINE}" >>${FILE_CHANGELOG}
    echo "" >>${FILE_CHANGELOG}
  done <<<"${FIX_PR_EN}"
  echo "***" >>${FILE_CHANGELOG}
  echo "" >>${FILE_CHANGELOG}
fi

if [ -n "${FIX_PR_ZH}" ]; then
  echo "## 问题修复" >>${ZH_CN_FILE_CHANGELOG}
  echo "" >>${ZH_CN_FILE_CHANGELOG}
  while read LINE; do
    echo "${LINE}" >>${ZH_CN_FILE_CHANGELOG}
    echo "" >>${ZH_CN_FILE_CHANGELOG}
  done <<<"${FIX_PR_ZH}"
  echo "***" >>${ZH_CN_FILE_CHANGELOG}
  echo "" >>${ZH_CN_FILE_CHANGELOG}
fi

#
for changelog_file in "${FILE_CHANGELOG}" "${ZH_CN_FILE_CHANGELOG}"; do
  echo "## Total " >>"${changelog_file}"
  echo "" >>"${changelog_file}"
  echo "Pull request number: ${TOTAL_COUNT}" >>"${changelog_file}"
  echo "" >>"${changelog_file}"
  echo "[ Commits ](https://github.com/${PROJECT_REPO}/compare/${START_TAG}...${DEST_TAG})" >>"${changelog_file}"
done

echo "--------------------"
echo "generate changelog to ${FILE_CHANGELOG} and ${ZH_CN_FILE_CHANGELOG}"
