# Proxy


This package provides a simple proxy server that can be used to proxy requests to a different server.

We use fly.io to test this package. You can find the fly.toml file in the root of this repository.

There is a server written in Golang and a client written in rust. The client
sends a request to the server on a given port and the server proxies the request to a different server.
