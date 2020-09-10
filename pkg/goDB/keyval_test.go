package goDB

import (
	"testing"

	jsoniter "github.com/json-iterator/go"
)

func TestJSONMarshalAggFlowMap(t *testing.T) {

	m := AggFlowMap{
		Key{Protocol: 0x11}: &Val{1, 1, 0, 0},
		Key{Protocol: 0x06}: &Val{2, 2, 0, 0},
	}

	b, err := jsoniter.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal aggregated flow map: %s", err)
	}
	_ = b
}
