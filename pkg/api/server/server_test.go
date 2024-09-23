package server

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateOpenAPISpec(t *testing.T) {
	buf := &bytes.Buffer{}
	s := NewDefault("test", "localhost:8146")

	err := s.WriteOpenAPISpec(buf)
	require.Nil(t, err)
}
