# pool_checkr

monitor mining.notify stratum logs for coinbase information

## Getting started

Pull the source code down and build the Go binary [or use a docker image](https://hub.docker.com/r/adamdecaf/bitaxe-coinbase-checker).

## Usage

pool_checkr offers a Go binary and docker image for running.

## Go binary

```
go run ./cmd/bitaxe-coinbase-checker -addresses 192.168.12.52
```
```
2026/01/22 11:35:51 INFO: connecting to ws://192.168.12.52:80/api/ws
2026/01/22 11:35:55 INFO: height=933400 has 2 coinbase outputs
2026/01/22 11:35:55 INFO: bc1qce93hy5rhg02s6aeu7mfdvxg76x66pqqtrvzs3 (P2WPKH) receives 3.13710696 BTC
```

## Docker image

```
docker run adamdecaf/bitaxe-coinbase-checker:v0.2.1 -addresses <ip:port> -expected <wallet-address>
```
```
2026/01/22 18:13:00 INFO: connecting to ws://192.168.12.52:80/api/ws
2026/01/22 18:13:00 INFO: height=933401 has 2 coinbase outputs
2026/01/22 18:13:00 INFO: bc1qce93hy5rhg02s6aeu7mfdvxg76x66pqqtrvzs3 (P2WPKH) receives 3.17368819 BTC
2026/01/22 18:13:00 INFO: all 1 addresses expected are in coinbase output
```

# Credits

Shoutout to [skot's pool_checkr](https://github.com/skot/pool_checkr) repository and [web version](https://skot.github.io/pool_checkr/) for the inspiration to learn and realize `mining.notify` can be easily parsed.

# License

MIT
