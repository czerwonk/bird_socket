# bird_socket
[![GoDoc](https://godoc.org/github.com/czerwonk/bird_socket?status.svg)](https://godoc.org/github.com/czerwonk/bird_socket)
[![Build Status](https://travis-ci.org/czerwonk/bird_socket.svg)](https://travis-ci.org/czerwonk/bird_socket)
[![Go Report Card](https://goreportcard.com/badge/github.com/czerwonk/bird_socket)](https://goreportcard.com/report/github.com/czerwonk/bird_socket)
[![Sourcegraph](https://sourcegraph.com/github.com/czerwonk/bird_socket/-/badge.svg)](https://sourcegraph.com/github.com/czerwonk/bird_socket?badge)

Golang library to communicate with Bird routing daemon

## Usage

The package keeps the original `Query` API and applies bounded defaults: a
30-second operation timeout and a 64 MiB response limit.

```go
reply, err := birdsocket.Query("/run/bird/bird.ctl", "show status")
```

Callers that already have a request or scrape context should use
`QueryContext` and set limits for their workload:

```go
reply, err := birdsocket.QueryContext(
	ctx,
	"/run/bird/bird.ctl",
	"show protocols all",
	birdsocket.WithTimeout(5*time.Second),
	birdsocket.WithMaxResponseBytes(4<<20),
)
```

Terminal `8xxx` and `9xxx` replies are returned as `*ReplyError`.
`ErrResponseTooLarge` can be checked with `errors.Is`.

## License
(c) Daniel Czerwonk, 2017. Licensed under [MIT](LICENSE) license.

## Bird routing daemon
see http://bird.network.cz/
