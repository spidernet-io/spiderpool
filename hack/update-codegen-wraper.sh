#!/bin/bash
set -o errexit
set -o nounset
set -o pipefail

CURRENT_FILENAME=`basename $0`
CURRENT_DIR_PATH=$(cd `dirname $0`; pwd)

clog(){
    echo "[ $CURRENT_FILENAME ]: $@"
}

# source: pkg/apis/spiderpool/v1 , will generate to pkg/apis/spiderpool/v1/generated
CRD_LIST=(
    "pkg/apis:spiderpool:v1"
    "pkg/apis:spiderpool:v2"
)

for ITEM in ${CRD_LIST[@]} ; do
    INPUT_PATH_BASE=` echo "$ITEM" | awk -F: '{print $1}'`
    API_OPERATOR_NAME=` echo "$ITEM" | awk -F: '{print $2}'`
    API_VERSION=` echo "$ITEM" | awk -F: '{print $3}'`
    [ -z "$INPUT_PATH_BASE" ] && clog "error, empty INPUT_PATH_BASE" && exit 1
    [ -z "$API_OPERATOR_NAME" ] && clog "error, empty API_OPERATOR_NAME" && exit 1
    [ -z "$API_VERSION" ] && clog "error, empty API_VERSION" && exit 1
    export INPUT_PATH_BASE
    export API_OPERATOR_NAME
    export API_VERSION
    clog "generate for ${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION}"
    ${CURRENT_DIR_PATH}/update-codegen.sh
    (($?!=0)) && clog "error, failed to execute update-codegen.sh for ${INPUT_PATH_BASE}/${API_OPERATOR_NAME}/${API_VERSION} " && exit 2
done

exit 0
