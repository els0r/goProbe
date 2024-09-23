package server

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateOpenAPISpec(t *testing.T) {
	buf := &bytes.Buffer{}
	s := New("localhost:8146", nil, nil)

	err := s.WriteOpenAPISpec(buf)
	require.Nil(t, err)
}
