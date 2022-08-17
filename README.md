# libp2p-playground

### MDNS pub/sub service
```bash
$ go run ./cmd/mdnspeer
# Another terminal
$ go run ./cmd/mdnspeer
```

### DHT pub/sub service
```bash
$ go run ./cmd/dhtbootstrap
# Another terminal
$ go run ./cmd/dhtbootstrap
```

### memberlist service
```bash
$ go run ./cmd/memberlistpeer
# ${addr_output}
# Another terminal
$ go run ./cmd/memberlistpeer -members=${addr_output}
```
