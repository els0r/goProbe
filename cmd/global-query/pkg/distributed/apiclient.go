package distributed

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"log/slog"

	"github.com/els0r/goProbe/cmd/global-query/pkg/hosts"
	"github.com/els0r/goProbe/pkg/api/goprobe/client"
	"github.com/els0r/goProbe/pkg/query"
	"github.com/els0r/goProbe/pkg/results"
	"github.com/els0r/telemetry/logging"
	"gopkg.in/yaml.v3"
)

// APIClientQuerier implements an API-based querier, fulfilling the Querier interface
type APIClientQuerier struct {
	apiEndpoints map[string]*client.Config

	maxConcurrent int
}

// one CPU can handle more than one client call at a time
var defaultMaxConcurrent = 2 * runtime.NumCPU()

// NewAPIClientQuerier instantiates a new goProbe API-based querier. It uses the goprobe/client
// under the hood to run queries
func NewAPIClientQuerier(cfgPath string) (*APIClientQuerier, error) {
	a := &APIClientQuerier{
		apiEndpoints:  make(map[string]*client.Config),
		maxConcurrent: defaultMaxConcurrent,
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

	err = yaml.NewDecoder(f).Decode(a.apiEndpoints)
	if err != nil {
		return nil, fmt.Errorf("failed to read in config: %w", err)
	}
	return a, nil
}

// SetMaxConcurrent guides how many hosts are contacted to run queries concurrently. In most cases,
// it's sufficient to set this to the amount of hosts available in configuration or the list of hosts
// queried
func (a *APIClientQuerier) SetMaxConcurrent(num int) *APIClientQuerier {
	a.maxConcurrent = num
	return a
}

// createQueryWorkload prepares and executes the workload required to perform the query
func (a *APIClientQuerier) createQueryWorkload(_ context.Context, host string, args *query.Args) (*queryWorkload, error) {
	qw := &queryWorkload{
		Host: host,
		Args: args,
	}
	// create the api client runner by looking up the endpoint config for the given host
	cfg, exists := a.apiEndpoints[host]
	if !exists {
		err := fmt.Errorf("couldn't find endpoint configuration for host")

		// inject an error runner so that the workload creation error is transported into the final
		// result
		qw.Runner = &errorRunner{err: err}
	} else {
		qw.Runner = client.NewFromConfig(cfg)
	}

	return qw, nil
}

// prepareQueries creates query workloads for all hosts in the host list and returns the channel it sends the
// workloads on
func (a *APIClientQuerier) prepareQueries(ctx context.Context, hostList hosts.Hosts, args *query.Args) <-chan *queryWorkload {
	workloads := make(chan *queryWorkload)

	go func(ctx context.Context) {
		logger := logging.FromContext(ctx)

		for _, host := range hostList {
			wl, err := a.createQueryWorkload(ctx, host, args)
			if err != nil {
				logger.With("hostname", host).Errorf("failed to create workload: %v", err)
			}
			workloads <- wl
		}
		close(workloads)
	}(ctx)

	return workloads
}

// AllHosts returns a list of all hosts / targets available to the querier
func (a *APIClientQuerier) AllHosts() (hostList hosts.Hosts, err error) {
	hostList = make([]string, 0, len(a.apiEndpoints))
	for host := range a.apiEndpoints {
		hostList = append(hostList, host)
	}

	return
}

// Query takes query workloads from the internal workloads channel, runs them, and returns a channel from which
// the results can be read
func (a *APIClientQuerier) Query(ctx context.Context, hosts hosts.Hosts, args *query.Args) <-chan *results.DistributedResult {
	out := make(chan *results.DistributedResult, a.maxConcurrent)

	workloads := a.prepareQueries(ctx, hosts, args)

	// query pipeline setup
	// sets up a fan-out, fan-in query processing pipeline
	numRunners := len(hosts)
	if 0 < a.maxConcurrent && a.maxConcurrent < numRunners {
		numRunners = a.maxConcurrent
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

					res, err := wl.Runner.Run(ctx, wl.Args)
					if err != nil {
						err = fmt.Errorf("failed to run query: %w", err)
					}

					qr := &results.DistributedResult{
						Hostname: wl.Host,
						Result:   res,
						Error:    err,
					}

					out <- qr
				}
			}
		}(ctx)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// queryWorkload denotes an individual workload to perform a query on a remote host
type queryWorkload struct {
	Host string

	Runner query.Runner
	Args   *query.Args
}
