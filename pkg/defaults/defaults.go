package defaults

import "time"

const (

	// DBPath denotes an FHS-compliant example path on disk where the DB can reside.
	DBPath = "/var/lib/goprobe/db"

	// QueryTimeout denotes the default query timeout
	QueryTimeout = 0 * time.Second
)
