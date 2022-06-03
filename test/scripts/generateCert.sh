#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail


OUTPUT_DIR="$1"
[ -z "$OUTPUT_DIR" ] && echo "error, miss OUTPUT_DIR" >&2 && exit 1

if [ -d "$OUTPUT_DIR" ] ;then
  rm -rf ${OUTPUT_DIR}/* || true
else
  mkdir -p ${OUTPUT_DIR}
fi

cd ${OUTPUT_DIR}

#-------------
CommonName="spidernet"

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
IP.1    = 192.168.1.8
DNS.1   = spiderpool.spidernet.io
DNS.2   = *.spidernet.io
EOF

#server key
openssl genrsa -out server.key 2048

# Generate a Certificate Signing Request (CSR) for the private key, and sign it with the private key of the CA.
openssl req -new -key server.key -config server.conf \
    | openssl x509 -req -CA ca.crt -CAkey ca.key -extensions v3_req -extfile server.conf -days 3650 -CAcreateserial -out server.crt

rm -f server.conf
rm -f ca.srl

echo "succeed to generate certifacte to $OUTPUT_DIR "
