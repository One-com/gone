# gone
Golang packages for writing small daemons and servers.

This is not strictly a "framework". The individual packages do not really depend on each other and can be used independently. However, they work well together.

## packages

* **log** is a drop-in replacement for the standard Go logging library "log" which is fully source code compatible support all the standard library API while at the same time offering advanced logging features through an extended API.

* **sd** Manages your socket file descriptors and (if wanted) interacts with Linux systemd socket-activation, FDSTORE and NOTIFY socket - and provides process management if you want the old style fork/kill process replacement reload.

* **http** Provides extentions of the standard HTTP library. A Server capable of graceful shutdown and a client side failover virtual Transport

* **daemon** wraps the sd package and a lot of daemon management boilerplate code to make if very easy to start a full featured daemon, doing graceful reload and/or zero-downtime restart/upgrades.

... more to come.

