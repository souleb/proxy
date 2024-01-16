# Proxy

A raw TCP proxy, configured with multiple apps to proxy, each with potentially 
many backend targets.

## Usage

### Build

```shell
$ go build -o proxy main.go
```

### Run

```shell
$ ./proxy --help
Usage of ./proxy:
      --config string     config file containing apps (default "config.json")
      --max-workers int   maximum number of workers (default 50)
```