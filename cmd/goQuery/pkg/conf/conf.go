package conf

const (
	queryKey = "query"

	serverKey       = queryKey + ".server"
	QueryServerAddr = serverKey + ".addr"
	QueryTimeout    = queryKey + ".timeout"

	dbKey       = "db"
	QueryDBPath = dbKey + ".path"

	StoredQuery = "stored-query"
)
