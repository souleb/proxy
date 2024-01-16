# TCP PROXY

## Design

The `TCP proxy` is a simple TCP proxy that accepts connection on a list of `ports` and 
forwards them to a list of `targets`.

### configuration

The proxy load a configuration file that contains `Apps`. Each `App` contains a list 
of `ports` and a list of `targets`.
The file path is configurable with the flag `--config-file`. The default value is `config.json`.

### forwarding connections

For every `port`, the proxy will listen on the `port` in a separate goroutine. For
every connection, the proxy will select a `target` in a round-robin fashion and
forward the connection to the `target`. We loop over the list of `targets` in 
case of `Dial` error and mark the faulty targets as dead. Even if a target is 
marked dead, it will be used in the round-robin algorithm. In a future version, 
we would try to reconnect to dead `targets` after a given time.

### worker pool

All accepted connections are forwarded to a worker pool via a channel. The worker
pool is a pool of goroutines that will forward a given connection to the selected
`target`. The worker pool  is a fixed size pool. If the pool is full, it will 
block until a worker is available.

The pool size is configurable with the flag `--max-workers`. The default value is 50.

Using a worker pool allows us to limit the number of goroutines that are used to
forward connections. This is useful in order to limit the number of open file descriptors.
We could have pre-allocated a fixed number of goroutines  but as goroutines are
very cheap, we spin up a new goroutine for every connection. We use a semaphore 
to limit the number of goroutines that are spawned at any given time.

#### Handler

The handler is a function that is called for every connection. We use `io.Copy` 
to copy between connections as it uses the `splice syscall` on linux and is very efficient.

### Shutdown

When shutting down the proxy(CTRL+C), we close the listeners to stop accepting 
new connections. When all the listeners are closed, we close the worker pool to
stop forwarding connections, this means closing all active connections. We wait 
for all the workers to finish before exiting.

### Making this production ready

We do not validate the configuration file. A wrong configuration file could break
the proxy.

For production use, we should set limits on the number of apps, ports and targets.

We do not have a hot reload of the file config. We could register/deregister apps
based on that.

We could have a better round-robin algorithm that could take into account the number
of retries.

We do not try to reconnect to dead targets. We should implement a ping mechanism to
check if a target is alive and re-enable it alive.

A dockerfile would be useful to deploy the proxy as well as makefile to build the
proxy.

## Cluster

I would have a pool of proxy instances, each running the proxy.

I would put those instances behind a load balancer and the load balancer
would select and instance to forward the connection to.

The loadbalancer would perform healthchecks and if an instance is down, it would
remove it from the pool.

If using kubernetes, I would use a Deployment and a Service for that. For Nomad,
I would have to install a load balancer and use a group of proxy instances.

To make this global, at fly.io (because you have anycast addresses), I would have
a distributed pool of clusters and use anycast to forward the connection to the 
closest cluster.
Otherwise, geographic load balancing would be a good option on a cloud provider
like AWS.
