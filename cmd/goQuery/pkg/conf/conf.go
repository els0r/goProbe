package conf

const (
	queryKey = "query"

	serverKey       = queryKey + ".server"
	QueryServerAddr = serverKey + ".addr"

	dbKey       = "db"
	QueryDBPath = dbKey + ".path"

	DBPath      = "db-path"
	StoredQuery = "stored-query"
)
