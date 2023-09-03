package defaults

import "time"

const (

	// DBPath denotes the default path on disk where the DB resides
	DBPath = "/usr/local/goProbe/db"

	// QueryTimeout denotes the default query timeout
	QueryTimeout = 0 * time.Second
)
