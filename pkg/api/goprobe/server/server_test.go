package server

import (
	"context"
	"testing"

	"github.com/els0r/goProbe/v4/pkg/api/server"
	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/stretchr/testify/require"
)

// rejectingAuthorizer is an authorizer stub; the sensor server must refuse it before use
type rejectingAuthorizer struct{}

func (rejectingAuthorizer) Authorize(context.Context, authz.Request) (authz.Scope, error) {
	return nil, authz.ErrForbidden
}

func TestNew_RejectsAuthorizer(t *testing.T) {
	require.PanicsWithError(t, errAuthorizerUnsupported.Error(), func() {
		New("127.0.0.1:8145", t.TempDir(), nil, nil, server.WithAuthorizer(rejectingAuthorizer{}))
	})
}

func TestNew_WithoutAuthorizer(t *testing.T) {
	require.NotPanics(t, func() {
		New("127.0.0.1:8145", t.TempDir(), nil, nil)
	})
}
