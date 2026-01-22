FROM golang:1.25-alpine AS builder
RUN apk add --no-cache ca-certificates make tzdata

WORKDIR /src/app
COPY . .
RUN make build

FROM alpine:3
LABEL maintainer="Adam Shannon <adamkshannon@gmail.com>"
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /src/app/bin/bitaxe-coinbase-checker /bin/bitaxe-coinbase-checker
COPY --from=builder /etc/passwd /etc/passwd

ENTRYPOINT ["/bin/bitaxe-coinbase-checker"]
