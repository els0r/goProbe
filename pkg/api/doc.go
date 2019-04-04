// Package api provides the methods for goProbe's control server. It registers all versions of the API via the .../v{versionNumber}/ path.
//
// Base path: /
//
// Program metrics for instrumentation (GET)
//
// Path: /debug/vars
//
//     Returns all metrics exposed via the "expvar" library
//
// Path: api/v1/
//
//     Access to API version 1 functions
package api

var mdAPIDocIntro = `API documentation for goProbe (auto-generated). The default output is JSON. Results can be pretty-printed by passing URL parameter` + "`pretty=1`" + `

## Authentication

It is highly recommended to use the API with pre-shared keys in order to shield it from unwanted third-parties. To achieve this, a "keys" array can be provided in goProbe's configuration and the API will register these keys.

If this option is used, all requests _must_ set an authorization header in the form "Authorization: digest KEY". It is recommended to generate sha256sums and use those as API keys. The key has to be 32 characters or longer.

### Examples:
` + example("curl \\\n  -X GET \\\n  -H \"Authorization: digest 80870e361129738388e155fde745f5112e2d242916697907a4ccf041be5f5184\" \\\n  http://localhost:6060/api/v1/stats/packets?pretty=1") + `
` + docV1

var docV1 = `
# Version 1

## Queries

This API provides access to some of goProbe's inner working. The stats path is mainly there to query counters and statistics of the underlying pcap handle. Also, any errors encountered during packet decoding can be displayed.

To scrutinize the currently active flow map, the /flows/ path can be used. It will return the in-memory structure used to track flows. Note that the source port is part of the structure as source port aggregation is performed prior to DB writeout.

### Examples:

These examples assume that you are running the API server with the default settings (localhost:6060).

Pretty print all active flows for eth0
` + example("curl -X GET http://localhost:6060/api/v1/flows/eth0?pretty=1") + `

Get detailed pacp stats per interface
` + example("curl -X GET http://localhost:6060/api/v1/stats/packets?pretty=1&debug=1") + `

## Actions

Any supported action is prefixed with a "_". goProbe has support for live-reloading the capture configuration. The /_reload path comes in handy when adding/removing interfaces for capturing in place. Upon reload, goProbe will load the changes and adjust its capturing routines.

### Examples:
` + example("curl -X POST http://localhost:6060/api/v1/_reload") + `
`

func example(command string) string {
	return "```\n" + command + "\n```"
}
