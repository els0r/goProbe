package discovery

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	capconfig "github.com/els0r/goProbe/cmd/goProbe/config"
	"github.com/els0r/goProbe/pkg/version"
	log "github.com/els0r/log"
)

// Config stores an endpoint's API configuration, e.g. under which host it is reachable, it's authorization details and which API versions are supported.
type Config struct {
	Identifier string   `json:"identifier,omitempty"`
	Versions   []string `json:"versions,omitempty"`
	Endpoint   string   `json:"endpoint,omitempty"`
	Keys       []string `json:"keys,omitempty"`
}

// String implements the config in human-readable form
func (c *Config) String() string {
	keys := "none"
	if len(c.Keys) > 0 {
		keys = "[...]"
	}
	return fmt.Sprintf("{%s@%s; v=%v; keys=%s}", c.Identifier, c.Endpoint, c.Versions, keys)
}

// MakeConfig reads the relevant parts from goProbe's config and creates a discovery config from it. If the discovery section is nil, a nil config is returned.
func MakeConfig(cfg *capconfig.Config) *Config {

	if cfg.API.Discovery != nil {
		return &Config{
			Identifier: cfg.API.Discovery.Identifier,
			Keys:       cfg.API.Keys,
			Versions:   []string{"v1"}, // default is at least v1
			Endpoint:   cfg.API.Discovery.Endpoint,
		}
	}
	return nil
}

// ProbesClient describes a path under the NTM discovery API
type ProbesClient interface {

	// Create inserts a new config to the key-value store. A non-nil error is returned if the config could not be registered. The stored configuration is returned to check consistency.
	Create(cfg *Config) (*Config, error)

	// Update takes a configuration and updates the configuration for `identifier` in the key-value store. The stored configuration is returned to check consistency.
	Update(identifier string, cfg *Config) (*Config, error)

	// Delete deletes a configuration for a given identifier
	Delete(identifier string) error

	// Get fetches a configuration for a given identifier
	Get(identifier string) (*Config, error)

	// Get fetches all configurations
	List() ([]*Config, error)
}

// probesClient handles all resources under /probes
type probesClient struct {
	baseURL string
}

func newProbesClient() *probesClient {
	return &probesClient{baseURL: "probes"}
}

// is reported in user agnet
const (
	clientVersion = "0.1"
)

// Client implements all methods to talk to the ntm discovery service API
type Client struct {
	baseURL   string
	userAgent string

	probes *probesClient

	httpClient *http.Client
}

// Option allows to modify non-mandatory parameters of the client
type Option func(*Client)

// WithUserAgent allows to set a different user agent
func WithUserAgent(agent string) Option {
	return func(c *Client) {
		c.userAgent = agent
	}
}

// WithRequestTimeout allows to set a different request timeout
func WithRequestTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithAllowSelfSignedCerts skips SSL certificate checks
func WithAllowSelfSignedCerts() Option {
	return func(c *Client) {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		c.httpClient.Transport = tr
	}
}

// NewClient creates a new API client
func NewClient(host string, opts ...Option) *Client {

	if !(strings.Contains(host, "https://") || strings.Contains(host, "http://")) {
		host = "http://" + host
	}

	// create new client
	c := &Client{
		baseURL:    host,
		userAgent:  "goProbe/" + version.Version() + " discovery-client/" + clientVersion,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		probes:     newProbesClient(),
	}

	// apply options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) String() string {
	return fmt.Sprintf("base URL: %s; user-agent: %s; http-client: %v", c.baseURL, c.userAgent, *c.httpClient)
}

// ClientError provides more info about a failed API call
type ClientError struct {
	Status int
	Err    error
}

func (c *ClientError) Error() string {
	str := fmt.Sprintf("client error: %s", c.Err)
	if c.Status != 0 {
		str = fmt.Sprintf("%s (%d - %s)", str, c.Status, http.StatusText(c.Status))

	}
	return str
}

// Get retrieves a configuration from the ntm discovery service
func (c *Client) Get(identifier string) (*Config, error) {

	// create url path
	u := fmt.Sprintf("%s/%s/%s", c.baseURL, c.probes.baseURL, identifier)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, &ClientError{Err: err}
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, &ClientError{Err: err}
	}
	defer resp.Body.Close()

	// API specific logic
	if resp.StatusCode == http.StatusNotFound {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("probe config does not exist")}
	}

	respBody := struct {
		Message string  `json:"message"`
		Data    *Config `json:"data"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return nil, &ClientError{Status: resp.StatusCode, Err: err}
	}
	return respBody.Data, nil
}

func (c *Client) doRequest(req *http.Request) (*http.Response, error) {

	// set context
	ctx, cancel := context.WithTimeout(context.Background(), c.httpClient.Timeout)
	defer cancel()

	// set headers
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.WithContext(ctx)

	// get response
	return c.httpClient.Do(req)
}

// Create publishes a new configuration on the ntm discovery service
func (c *Client) Create(cfg *Config) (*Config, error) {
	// create url path
	u := fmt.Sprintf("%s/%s", c.baseURL, c.probes.baseURL)

	body := &bytes.Buffer{}
	// serialize config
	err := json.NewEncoder(body).Encode(cfg)
	if err != nil {
		return nil, &ClientError{Err: err}
	}

	var req *http.Request
	req, err = http.NewRequest("POST", u, body)
	if err != nil {
		return nil, &ClientError{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")

	// do request
	var resp *http.Response
	resp, err = c.doRequest(req)
	if err != nil {
		return nil, &ClientError{Err: err}
	}
	defer resp.Body.Close()

	// check response
	if resp.StatusCode == http.StatusNotFound {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("probe config does not exist")}
	}
	if !(resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK) {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("config creation failed: cfg=%s", cfg)}
	}

	// check mirrored config
	respBody := struct {
		Message string  `json:"message"`
		Data    *Config `json:"data"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("failed to read received config: %s", err)}
	}

	return respBody.Data, nil
}

// Update updates an existing configuration on the ntm discovery service
func (c *Client) Update(identifier string, cfg *Config) (*Config, error) {
	// create url path
	u := fmt.Sprintf("%s/%s/%s", c.baseURL, c.probes.baseURL, identifier)

	body := &bytes.Buffer{}
	// serialize config
	err := json.NewEncoder(body).Encode(cfg)
	if err != nil {
		return nil, &ClientError{Err: err}
	}

	var req *http.Request
	req, err = http.NewRequest("PUT", u, body)
	if err != nil {
		return nil, &ClientError{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")

	// do request
	var resp *http.Response
	resp, err = c.doRequest(req)
	if err != nil {
		return nil, &ClientError{Err: err}
	}
	defer resp.Body.Close()

	// check response
	if !(resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK) {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("create failed")}
	}

	// expected data
	respBody := struct {
		Message string  `json:"message"`
		Data    *Config `json:"data"`
	}{}

	// check mirrored config
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("failed to read received config: %s", err)}
	}
	if !reflect.DeepEqual(respBody.Data, cfg) {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("receive config: %s; create config: %s", respBody.Data, cfg)}
	}

	return respBody.Data, nil
}

// Delete deletes a configuration for a given identifier
func (c *Client) Delete(identifier string) error {
	// create url path
	u := fmt.Sprintf("%s/%s/%s", c.baseURL, c.probes.baseURL, identifier)

	req, err := http.NewRequest("DELETE", u, nil)
	if err != nil {
		return &ClientError{Err: err}
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return &ClientError{Err: err}
	}
	defer resp.Body.Close()

	// API specific logic
	if resp.StatusCode == http.StatusNotFound {
		return &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("probe config does not exist")}
	}
	return nil
}

// List lists all stored probe configurations
func (c *Client) List() ([]*Config, error) {
	// create url path
	u := fmt.Sprintf("%s", c.baseURL)

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, &ClientError{Err: err}
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, &ClientError{Err: err}
	}
	defer resp.Body.Close()

	// API specific logic
	if resp.StatusCode != http.StatusOK {
		return nil, &ClientError{Status: resp.StatusCode, Err: fmt.Errorf("could not list probe configs")}
	}

	// expected data
	respBody := struct {
		Message string    `json:"message"`
		Data    []*Config `json:"data"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&respBody)

	// the list can be empty which would result in an EOF error
	if err != nil && err != io.EOF {
		return nil, &ClientError{Status: resp.StatusCode, Err: err}
	}

	return respBody.Data, nil
}

type state int

const (
	acceptNew state = iota // the config hasn't been updated
	idle                   // configs are updated, nothing todo
)

type configUpdater struct {
	state      state
	config     *Config
	identifier string
}

func (c *configUpdater) handleConfig(client ProbesClient, logger log.Logger) {
	// a nil config triggers de-registration of the probe
	if c.config == nil {
		err := client.Delete(c.identifier)
		if err != nil {
			logger.Errorf("failed to deregister probe with id=%s", c.identifier)
		} else {
			logger.Infof("deregistered probe with id=%s", c.identifier)

			// accept new configs
			c.identifier = ""
			c.state = acceptNew
		}
		return
	}
	if c.state == acceptNew {
		err := RegisterOrUpdateConfig(client, c.config, logger)
		if err == nil {
			c.state = idle
			c.identifier = c.config.Identifier // store identifier for possible deregistration
		}
	}
}

// RunConfigRegistration listens for config updates and retries periodically to reach the discovery service and register the config
func RunConfigRegistration(client ProbesClient, logger log.Logger) chan *Config {

	updater := &configUpdater{
		state: acceptNew,
	}

	// allows for config updates
	cfgUpdateChan := make(chan *Config)

	// repeatedly try
	go func() {
		for {
			select {
			case <-time.After(5 * time.Minute):
				updater.handleConfig(client, logger)
			case cfg := <-cfgUpdateChan:
				updater.state = acceptNew
				updater.config = cfg

				updater.handleConfig(client, logger)
			}
		}
	}()

	return cfgUpdateChan
}

// RegisterOrUpdateConfig takes a configuration and registers it at the discovery service. If it exists already, it is updated.
func RegisterOrUpdateConfig(client ProbesClient, cfg *Config, logger log.Logger) error {
	cfgRec, err := client.Create(cfg)
	if err == nil {
		logger.Info("Successfully registered probe with discovery service")

		// check if config is the same, otherwise run an update
		if !reflect.DeepEqual(cfgRec, cfg) {
			_, err := client.Update(cfgRec.Identifier, cfg)
			if err == nil {
				logger.Info("Updated discovery configuration")
			} else {
				logger.Errorf("Probe discovery configuration update failed: %s", err)
			}
			return err
		}
	} else {
		logger.Errorf("Probe registration failed: %s", err)
	}
	return err
}
