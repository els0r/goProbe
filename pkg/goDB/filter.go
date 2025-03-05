package goDB

import (
	"github.com/els0r/goProbe/v4/pkg/types/hashmap"
)

// FilterFn denotes a function that filters an exisiting AggFlowMap and returns a new one
// Note: In case of a null-op, the output map may be the same as the input map
type FilterFn func(*hashmap.AggFlowMap) *hashmap.AggFlowMap

// QueryFilter returns a FilterFn that applies a query condition to an existing AggFlowMap
func QueryFilter(query *Query) FilterFn {
	return func(input *hashmap.AggFlowMap) (result *hashmap.AggFlowMap) {

		// If there is no condition, return the input map as is
		if query.Conditional == nil {
			return input
		}

		result = hashmap.NewAggFlowMap()

		// Loop over primary (IPv4) entries
		for it := input.PrimaryMap.Iter(); it.Next(); {
			if conditionalSatisfied := query.Conditional.Evaluate(it.Key()); conditionalSatisfied {
				result.PrimaryMap.SetOrUpdate(it.Key(),
					it.Val().BytesRcvd,
					it.Val().BytesSent,
					it.Val().PacketsRcvd,
					it.Val().PacketsSent,
				)
			}
		}

		// Loop over primary (IPv6) entries
		for it := input.SecondaryMap.Iter(); it.Next(); {
			if conditionalSatisfied := query.Conditional.Evaluate(it.Key()); conditionalSatisfied {
				result.SecondaryMap.SetOrUpdate(it.Key(),
					it.Val().BytesRcvd,
					it.Val().BytesSent,
					it.Val().PacketsRcvd,
					it.Val().PacketsSent,
				)
			}
		}

		return
	}
}
