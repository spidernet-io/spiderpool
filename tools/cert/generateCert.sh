#!/bin/bash

# Copyright 2022 Authors of spidernet-io
# SPDX-License-Identifier: Apache-2.0

set -o errexit
set -o nounset
set -o pipefail

OUTPUT_DIR="$1"
[ -z "$OUTPUT_DIR" ] && echo "error, miss OUTPUT_DIR" >&2 && exit 1

mkdir -p ${OUTPUT_DIR}
rm -rf ${OUTPUT_DIR:?}/* || true

cd ${OUTPUT_DIR}

#-------------
# the https server, visited by service, so use service dns as CN
serviceName=${serviceName:-"spiderpool-controller"}
nameSpace=${nameSpace:-"kube-system"}
clusterDomain=${clusterDomain:-"cluster.local"}

CommonName="${serviceName}.${nameSpace}.svc"

# CA cert
openssl req -nodes -new -x509 -keyout ca.key -out ca.crt -subj "/CN=${CommonName}" -days 3650


cat >server.conf <<EOF
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
prompt = no

[req_distinguished_name]
CN = ${CommonName}

[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth, serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1   = ${serviceName}
DNS.2   = ${serviceName}.${nameSpace}
DNS.3   = ${serviceName}.${nameSpace}.svc
DNS.4   = ${serviceName}.${nameSpace}.svc.${clusterDomain}
EOF

#server key
openssl genrsa -out server.key 2048

# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
openssl req -new -key server.key -config server.conf \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -extensions v3_req -extfile server.conf -days 3650 -CAcreateserial -out server.crt

rm -f server.conf
rm -f ca.srl

echo "succeed to generate certificate for ${CommonName} to directory $OUTPUT_DIR "
