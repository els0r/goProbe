package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	log "github.com/els0r/log"
)

func (k APIKeys) Exists(key string) bool {
	_, exists := k[key]
	return exists
}

func checkKeyConstraints(key string) error {
	// enforce long API keys (e.g. SHA256)
	if len(key) < 32 {
		return fmt.Errorf("key considered insecure: insufficient key length %d", len(key))
	}

	// TODO: consider to check entropy of key
	return nil
}

// authenticator implements the middleware http.Handler
type authenticator struct {
	h      http.Handler
	keys   APIKeys
	logger log.Logger
}

// ServeHTTP checks a request for valid authorization parameters and calls the next handler if successful
func (a *authenticator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		authCtx context.Context
		userKey string
	)

	ctx := r.Context()
	select {
	case <-ctx.Done():
		http.Error(w, errContextCanceled, http.StatusServiceUnavailable)
		return
	default:
		// get the authorization header of the request
		authHeader := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
		if len(authHeader) != 2 || authHeader[0] != "digest" {
			a.logger.Infof("user key denied: invalid auth header")

			ReturnStatus(w, http.StatusUnauthorized)
			return
		}
		userKey = authHeader[1]

		if !a.keys.Exists(userKey) {
			a.logger.Debugf("user key denied: not registered")

			ReturnStatus(w, http.StatusUnauthorized)
			return
		}

		// enforce key constraints
		err = checkKeyConstraints(userKey)
		if err != nil {
			a.logger.Infof("user key denied: %s", err)

			ReturnStatus(w, http.StatusPreconditionFailed)
			return
		}

		a.logger.Debugf("user successfully authenticated")

		// set key in request context
		authCtx = context.WithValue(r.Context(), apiKeyCtxKey, userKey)

		// call next handler
		a.h.ServeHTTP(w, r.WithContext(authCtx))
	}
}

// AuthenticationHandler registers API authentication keys and returns a middleware that checks them
func (s *Server) AuthenticationHandler(keys APIKeys) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return &authenticator{h: next, keys: keys, logger: s.logger}
	}
}
