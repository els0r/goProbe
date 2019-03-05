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
