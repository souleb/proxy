# Client

This is a simple client for the [proxy](../proxy/README.md).

The client connects to the proxy `address:port` and redirects the `stdin` to the proxy.
It also redirects the proxy's response to `stdout` in a non-blocking way (different
thread with tokio).)

## Usage

### Build

```shell
$ cargo build
   Compiling client v0.1.0 (file:///home/.../client)
    Finished dev [unoptimized + debuginfo] target(s) in 0.0 secs
```

### Run
```shell
$ cargo run -- --help
    Finished dev [unoptimized + debuginfo] target(s) in 0.0 secs
     Running `target/debug/client --help`
````
