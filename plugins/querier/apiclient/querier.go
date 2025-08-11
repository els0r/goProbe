package apiclient

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"log/slog"

	gqclient "github.com/els0r/goProbe/v4/pkg/api/globalquery/client"

	"github.com/els0r/goProbe/v4/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/v4/pkg/distributed"
	"github.com/els0r/goProbe/v4/pkg/distributed/hosts"
	"github.com/els0r/goProbe/v4/pkg/query"
	"github.com/els0r/goProbe/v4/pkg/results"
	"github.com/els0r/goProbe/v4/plugins"
	"github.com/els0r/telemetry/logging"
	"gopkg.in/yaml.v3"
)

const (
	// Name is the name of the API Client Querier plugin
	Name = "api"
)

func init() {
	plugins.RegisterQuerier(Name, func(_ context.Context, cfgPath string) (distributed.Querier, error) {
		return New(cfgPath)
	})
}

// APIClientQuerier implements an API-based querier, fulfilling the Querier interface
type APIClientQuerier struct {
	APIEndpoints  map[string]*client.Config `json:"endpoints" yaml:"endpoints"`
	MaxConcurrent int                       `json:"max_concurrent" yaml:"max_concurrent"`
}

// one CPU can handle more than one client call at a time
var defaultMaxConcurrent = 2 * runtime.NumCPU()

// New instantiates a new goProbe API-based querier. It uses the goprobe/client
// under the hood to run queries
func New(cfgPath string) (*APIClientQuerier, error) {
	a := &APIClientQuerier{
		APIEndpoints:  make(map[string]*client.Config),
		MaxConcurrent: defaultMaxConcurrent,
	}

	// read in the endpoints config
	f, err := os.Open(filepath.Clean(cfgPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open config: %w", err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	err = yaml.NewDecoder(f).Decode(a.APIEndpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to read in config: %w", err)
	}
	return a, nil
}

// SetMaxConcurrent guides how many hosts are contacted to run queries concurrently. In most cases,
// it's sufficient to set this to the amount of hosts available in configuration or the list of hosts
// queried
func (a *APIClientQuerier) SetMaxConcurrent(num int) *APIClientQuerier {
	a.MaxConcurrent = num
	return a
}

// createQueryWorkload prepares and executes the workload required to perform the query
func (a *APIClientQuerier) createQueryWorkload(_ context.Context, host string, args *query.Args, keepaliveChan chan struct{}) (*queryWorkload, error) {
	qw := &queryWorkload{
		Host: host,
		Args: args,
	}
	// create the api client runner by looking up the endpoint config for the given host
	cfg, exists := a.APIEndpoints[host]
	if !exists {
		err := fmt.Errorf("couldn't find endpoint configuration for host")

		// inject an error runner so that the workload creation error is transported into the final
		// result
		qw.Runner = distributed.NewErrorRunner(err)
	} else {
		if args.KeepAlive > 0 {
			qw.Runner = gqclient.NewSSEFromConfig(cfg, keepaliveChan)
		} else {
			qw.Runner = client.NewFromConfig(cfg)
		}
	}

	return qw, nil
}

// prepareQueries creates query workloads for all hosts in the host list and returns the channel it sends the
// workloads on
func (a *APIClientQuerier) prepareQueries(ctx context.Context, hostList hosts.Hosts, args *query.Args) (<-chan *queryWorkload, chan struct{}) {
	workloads := make(chan *queryWorkload)
	keepaliveChan := make(chan struct{}, 64)

	go func(ctx context.Context) {
		logger := logging.FromContext(ctx)

		for _, host := range hostList {
			wl, err := a.createQueryWorkload(ctx, string(host), args, keepaliveChan)
			if err != nil {
				logger.With("hostname", host).Errorf("failed to create workload: %v", err)
			}
			workloads <- wl
		}
		close(workloads)
	}(ctx)

	return workloads, keepaliveChan
}

// AllHosts returns a list of all hosts / targets available to the querier
func (a *APIClientQuerier) AllHosts() (hostList hosts.Hosts, err error) {
	hostList = make(hosts.Hosts, 0, len(a.APIEndpoints))
	for host := range a.APIEndpoints {
		hostList = append(hostList, hosts.ID(host))
	}

	return
}

// Query takes query workloads from the internal workloads channel, runs them, and returns a channel from which
// the results can be read and a channel from which keepalive signals can be read
func (a *APIClientQuerier) Query(ctx context.Context, hosts hosts.Hosts, args *query.Args) (<-chan *results.Result, <-chan struct{}) {
	out := make(chan *results.Result, a.MaxConcurrent)

	workloads, keepaliveChan := a.prepareQueries(ctx, hosts, args)

	// query pipeline setup
	// sets up a fan-out, fan-in query processing pipeline
	numRunners := len(hosts)
	if 0 < a.MaxConcurrent && a.MaxConcurrent < numRunners {
		numRunners = a.MaxConcurrent
	}

	logger := logging.FromContext(ctx).With("runners", numRunners)
	logger.Info("running queries")

	wg := new(sync.WaitGroup)
	wg.Add(numRunners)
	for i := 0; i < numRunners; i++ {
		go func(ctx context.Context) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case wl, open := <-workloads:
					if !open {
						return
					}

					ctx := logging.WithFields(ctx, slog.String("host", wl.Host))

					qr, err := wl.Runner.Run(ctx, wl.Args)
					if err != nil {
						qr = results.New()
						qr.SetErr(err)

						err = fmt.Errorf("failed to run query: %w", err)
					}
					qr.Hostname = wl.Host

					out <- qr
				}
			}
		}(ctx)
	}
	go func() {
		wg.Wait()
		close(out)
		if keepaliveChan != nil {
			close(keepaliveChan)
		}
	}()
	return out, keepaliveChan
}

// queryWorkload denotes an individual workload to perform a query on a remote host
type queryWorkload struct {
	Host string

	Runner        query.Runner
	Args          *query.Args
	KeepaliveChan chan struct{}
}
