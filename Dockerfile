FROM golang:1.25 as builder
WORKDIR /src/app
COPY . .
RUN make build

FROM scratch
COPY --from=builder /src/app/bin/bitaxe-coinbase-checker /bin/bitaxe-coinbase-checker
ENTRYPOINT ["/bin/bitaxe-coinbase-checker"]
