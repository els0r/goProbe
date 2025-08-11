package stringresolver

import (
	"context"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/stretchr/testify/require"
)

func TestResolver_Sorted(t *testing.T) {
	r := &Resolver{sorted: true}
	got, err := r.Resolve(context.Background(), "b, a, c, a,,b ,  d ")
	require.Nil(t, err, "Resolve should not return an error")

	want := hosts.Hosts{"a", "b", "c", "d"}
	require.Equal(t, want, got)
}

func TestResolver_Unsorted(t *testing.T) {
	r := &Resolver{sorted: false}
	// Unsorted output is map-iteration order; just assert set equality and length.
	got, err := r.Resolve(context.Background(), "x,x,y,,z, y ")
	require.Nil(t, err, "Resolve should not return an error")
	require.Len(t, got, 3, "expected 3 unique hosts")

	// Convert to set for stable checks
	have := map[string]struct{}{}
	for _, h := range got {
		have[h] = struct{}{}
	}
	for _, exp := range []string{"x", "y", "z"} {
		_, ok := have[exp]
		require.True(t, ok, "missing expected host %q in %v", exp, got)
	}
}

func TestResolver_EmptyInput(t *testing.T) {
	r := &Resolver{sorted: true}
	got, err := r.Resolve(context.Background(), "   , , ")
	require.Nil(t, err, "Resolve should not return an error")
	require.Equal(t, len(got), 0, "expected empty result")
}
