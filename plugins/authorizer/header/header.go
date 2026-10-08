// Package header implements the in-tree authorizer that reads the scope of a request from a
// header set by a trusted gateway. It is exactly as safe as the network path in front of it:
// the gateway must strip or replace the scope header on every client-reachable path
package header

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/els0r/goProbe/v4/pkg/distributed/authz"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/plugins"
	"gopkg.in/yaml.v3"
)

const (
	// Type is the name the authorizer is registered under
	Type = "header"

	// DefaultScopeHeader is the header carrying the comma-separated host IDs of the scope
	DefaultScopeHeader = "X-GoProbe-Allowed-Hosts"

	// DefaultPrincipalHeader is the header carrying the principal
	DefaultPrincipalHeader = "X-GoProbe-Principal"

	// UnknownPrincipal is reported when the principal header is absent or empty
	UnknownPrincipal = "unknown"

	// MaxPrincipalLength is the number of runes a principal is capped at before it is stored
	// and logged
	MaxPrincipalLength = 256
)

func init() {
	plugins.RegisterAuthorizer(Type, func(_ context.Context, cfgPath string) (authz.Authorizer, error) {
		return New(cfgPath)
	})
}

// Config is the file schema of the header authorizer
type Config struct {
	// Trusted acknowledges that clients cannot set the scope header. The authorizer refuses
	// to initialise unless it is true
	Trusted bool `yaml:"trusted"`

	// ScopeHeader names the header carrying the scope. Empty: DefaultScopeHeader
	ScopeHeader string `yaml:"scope_header"`

	// PrincipalHeader names the header carrying the principal. Empty: DefaultPrincipalHeader
	PrincipalHeader string `yaml:"principal_header"`
}

// Authorizer reads the scope and the principal of a request from gateway-set headers
type Authorizer struct {
	scopeHeader     string
	principalHeader string
}

// New loads the config file at cfgPath and returns the authorizer it describes
func New(cfgPath string) (*Authorizer, error) {
	if cfgPath == "" {
		return nil, errors.New("header authorizer: config file required: the trusted acknowledgement cannot be given without one")
	}
	b, err := os.ReadFile(filepath.Clean(cfgPath))
	if err != nil {
		return nil, fmt.Errorf("header authorizer: failed to read config: %w", err)
	}
	// unknown keys fail startup: a misspelled header name must not silently fall back to the
	// default
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	var cfg Config
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("header authorizer: failed to parse config: %w", err)
	}
	// an empty file decodes to the zero config, which fails the trusted check below
	return NewFromConfig(cfg)
}

// NewFromConfig returns the authorizer described by cfg
func NewFromConfig(cfg Config) (*Authorizer, error) {
	if !cfg.Trusted {
		return nil, errors.New("header authorizer: config must set trusted: true, acknowledging that the gateway strips or replaces the scope header on every client-reachable path so that clients cannot set it")
	}
	return &Authorizer{
		scopeHeader:     headerName(cfg.ScopeHeader, DefaultScopeHeader),
		principalHeader: headerName(cfg.PrincipalHeader, DefaultPrincipalHeader),
	}, nil
}

// headerName returns the trimmed configured header name, or def when it is blank
func headerName(configured, def string) string {
	if name := strings.TrimSpace(configured); name != "" {
		return name
	}
	return def
}

// Authorize turns the scope header into a Scope. A scope header that is missing, empty or
// present more than once is forbidden: a gateway that appends instead of replacing the header
// must not let a client-set value through
func (a *Authorizer) Authorize(_ context.Context, req authz.Request) (authz.Scope, error) {
	switch n := countHeader(req, a.scopeHeader); {
	case n == 0:
		return nil, fmt.Errorf("scope header %q missing: %w", a.scopeHeader, authz.ErrForbidden)
	case n > 1:
		return nil, fmt.Errorf("scope header %q present %d times: %w", a.scopeHeader, n, authz.ErrForbidden)
	}

	s := newScope(a.principal(req), req.Header(a.scopeHeader))
	if len(s.allowed) == 0 {
		return nil, fmt.Errorf("scope header %q empty: %w", a.scopeHeader, authz.ErrForbidden)
	}
	return s, nil
}

// principal returns the principal of the request. A principal header present more than once
// is reported as UnknownPrincipal: with an appending gateway the first value would be the
// client's, and the audit trail must not record it. The principal never influences the
// decision, so a repeated header is not rejected
func (a *Authorizer) principal(req authz.Request) string {
	if countHeader(req, a.principalHeader) > 1 {
		return UnknownPrincipal
	}
	return normalizePrincipal(req.Header(a.principalHeader))
}

// normalizePrincipal turns the principal header value into the reported principal: empty
// becomes UnknownPrincipal, anything longer than MaxPrincipalLength runes is cut
func normalizePrincipal(value string) string {
	if value == "" {
		return UnknownPrincipal
	}
	if runes := []rune(value); len(runes) > MaxPrincipalLength {
		return string(runes[:MaxPrincipalLength])
	}
	return value
}

// countHeader returns how many times the named header occurs, matched case-insensitively.
// Request.Header only returns the first value, so repetition is only visible this way
func countHeader(req authz.Request, name string) (n int) {
	req.EachHeader(func(hdr, _ string) {
		if strings.EqualFold(hdr, name) {
			n++
		}
	})
	return n
}

// scope is the set of host IDs listed in the scope header
type scope struct {
	principal string
	allowed   map[hosts.ID]struct{}
}

// newScope parses the comma-separated host IDs of value: each is trimmed, empties are dropped
func newScope(principal, value string) *scope {
	s := &scope{principal: principal, allowed: make(map[hosts.ID]struct{})}
	for _, id := range strings.Split(value, ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		s.allowed[hosts.ID(id)] = struct{}{}
	}
	return s
}

// Principal returns the principal reported by the principal header
func (s *scope) Principal() string { return s.principal }

// Filter returns the host IDs of hostIDs that are in the scope, in input order, matched exactly
func (s *scope) Filter(_ context.Context, hostIDs hosts.Hosts) (hosts.Hosts, error) {
	out := make(hosts.Hosts, 0, len(hostIDs))
	for _, id := range hostIDs {
		if _, ok := s.allowed[id]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}
