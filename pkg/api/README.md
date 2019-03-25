# github.com/els0r/goProbe

API documentation for goProbe (auto-generated). The default output is JSON. Results can be pretty-printed by passing URL parameter`pretty=1`

## Authentication

It is highly recommended to use the API with pre-shared keys in order to shield it from unwanted third-parties. To achieve this, a "keys" array can be provided in goProbe's configuration and the API will register these keys.

If this option is used, all requests _must_ set an authorization header in the form "Authorization: digest KEY". It is recommended to generate sha256sums and use those as API keys. The key has to be 32 characters or longer.

### Examples:
```
curl \
  -X GET \
  -H "Authorization: digest 80870e361129738388e155fde745f5112e2d242916697907a4ccf041be5f5184" \
  http://localhost:6060/api/v1/stats/packets?pretty=1
```

# Version 1

## Queries

This API provides access to some of goProbe's inner working. The stats path is mainly there to query counters and statistics of the underlying pcap handle. Also, any errors encountered during packet decoding can be displayed.

To scrutinize the currently active flow map, the /flows/ path can be used. It will return the in-memory structure used to track flows. Note that the source port is part of the structure as source port aggregation is performed prior to DB writeout.

### Examples:

These examples assume that you are running the API server with the default settings (localhost:6060).

Pretty print all active flows for eth0
```
curl -X GET http://localhost:6060/api/v1/flows/eth0?pretty=1
```

Get detailed pacp stats per interface
```
curl -X GET http://localhost:6060/api/v1/stats/packets?pretty=1&debug=1
```

## Actions

Any supported action is prefixed with a "_". goProbe has support for live-reloading the capture configuration. The /_reload path comes in handy when adding/removing interfaces for capturing in place. Upon reload, goProbe will load the changes and adjust its capturing routines.

### Examples:
```
curl -X POST http://localhost:6060/api/v1/_reload
```


## Routes

<details>
<summary>`/*/api/*/v1/*/*/_reload`</summary>

- [RequestID](https://github.com/go-chi/chi/middleware/request_id.go#L63)
- [RealIP](https://github.com/go-chi/chi/middleware/realip.go#L29)
- [RequestLogger.func1](https://github.com/go-chi/chi/middleware/logger.go#L36)
- [Recoverer](https://github.com/go-chi/chi/middleware/recoverer.go#L18)
- [Timeout.func1](https://github.com/go-chi/chi/middleware/timeout.go#L34)
- **/***
	- **/api/***
		- [github.com/throttled/throttled.(*HTTPRateLimiter).RateLimit-fm](/pkg/api/rate.go#L49)
		- [ThrottleBacklog.func1](https://github.com/go-chi/chi/middleware/throttle.go#L50)
		- [(*Server).AuthenticationHandler.func1](/pkg/api/auth.go#L86)
		- **/v1/***
			- **/***
				- **/_reload**
					- _POST_
						- [(*API).handleReload-fm](/pkg/api/v1/post_routes.go#L17)

</details>
<details>
<summary>`/*/api/*/v1/*/*/flows/*/{ifaceName}/*`</summary>

- [RequestID](https://github.com/go-chi/chi/middleware/request_id.go#L63)
- [RealIP](https://github.com/go-chi/chi/middleware/realip.go#L29)
- [RequestLogger.func1](https://github.com/go-chi/chi/middleware/logger.go#L36)
- [Recoverer](https://github.com/go-chi/chi/middleware/recoverer.go#L18)
- [Timeout.func1](https://github.com/go-chi/chi/middleware/timeout.go#L34)
- **/***
	- **/api/***
		- [github.com/throttled/throttled.(*HTTPRateLimiter).RateLimit-fm](/pkg/api/rate.go#L49)
		- [ThrottleBacklog.func1](https://github.com/go-chi/chi/middleware/throttle.go#L50)
		- [(*Server).AuthenticationHandler.func1](/pkg/api/auth.go#L86)
		- **/v1/***
			- **/***
				- **/flows/***
					- **/{ifaceName}/***
						- **/**
							- _GET_
								- [(*API).IfaceCtx-fm](/pkg/api/v1/get_routes.go#L28)
								- [(*API).getActiveFlows-fm](/pkg/api/v1/get_routes.go#L28)

</details>
<details>
<summary>`/*/api/*/v1/*/*/stats/*/errors`</summary>

- [RequestID](https://github.com/go-chi/chi/middleware/request_id.go#L63)
- [RealIP](https://github.com/go-chi/chi/middleware/realip.go#L29)
- [RequestLogger.func1](https://github.com/go-chi/chi/middleware/logger.go#L36)
- [Recoverer](https://github.com/go-chi/chi/middleware/recoverer.go#L18)
- [Timeout.func1](https://github.com/go-chi/chi/middleware/timeout.go#L34)
- **/***
	- **/api/***
		- [github.com/throttled/throttled.(*HTTPRateLimiter).RateLimit-fm](/pkg/api/rate.go#L49)
		- [ThrottleBacklog.func1](https://github.com/go-chi/chi/middleware/throttle.go#L50)
		- [(*Server).AuthenticationHandler.func1](/pkg/api/auth.go#L86)
		- **/v1/***
			- **/***
				- **/stats/***
					- **/errors**
						- _GET_
							- [(*API).getErrors-fm](/pkg/api/v1/get_routes.go#L22)

</details>
<details>
<summary>`/*/api/*/v1/*/*/stats/*/packets`</summary>

- [RequestID](https://github.com/go-chi/chi/middleware/request_id.go#L63)
- [RealIP](https://github.com/go-chi/chi/middleware/realip.go#L29)
- [RequestLogger.func1](https://github.com/go-chi/chi/middleware/logger.go#L36)
- [Recoverer](https://github.com/go-chi/chi/middleware/recoverer.go#L18)
- [Timeout.func1](https://github.com/go-chi/chi/middleware/timeout.go#L34)
- **/***
	- **/api/***
		- [github.com/throttled/throttled.(*HTTPRateLimiter).RateLimit-fm](/pkg/api/rate.go#L49)
		- [ThrottleBacklog.func1](https://github.com/go-chi/chi/middleware/throttle.go#L50)
		- [(*Server).AuthenticationHandler.func1](/pkg/api/auth.go#L86)
		- **/v1/***
			- **/***
				- **/stats/***
					- **/packets**
						- _GET_
							- [(*API).getPacketStats-fm](/pkg/api/v1/get_routes.go#L21)

</details>

Total # of routes: 4
