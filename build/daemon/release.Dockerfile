FROM golang:1.17.8 AS builder
WORKDIR /spiderpool
COPY . .
RUN make daemon

FROM alpine:3.15.0
COPY --from=builder /spiderpool/bin/spiderpool /opt/spidernet/bin/spiderpool
RUN chmod g+rwX /opt/spidernet/bin/spiderpool
RUN ln -s /opt/spidernet/bin/spiderpool /bin/spiderpool

USER 1001
ENTRYPOINT [ "spiderpool" ]
