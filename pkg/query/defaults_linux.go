// +build !arm 

package query

// MaxResults stores the maximum number of rows a query will return. This limit is more or less
// theoretical, since a DB will unlikley feature such an amount of entries
const MaxResults = 9999999999999999
