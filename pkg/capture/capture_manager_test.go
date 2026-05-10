package capture

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/els0r/goProbe/v4/cmd/goProbe/config"
	"github.com/fako1024/gotools/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/els0r/goProbe/v4/pkg/capture/capturetypes"
)

type noopWriteoutHandler struct{}

func (noopWriteoutHandler) HandleWriteout(context.Context, time.Time, <-chan capturetypes.TaggedAggFlowMap) <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

func TestAutodetectIfaces(t *testing.T) {
	cm := &Manager{}

	defaultCfg := config.DefaultCaptureConfig()

	stubLinks := link.Links{
		&link.Link{Name: "eth0"},
		&link.Link{Name: "eth1"},
		&link.Link{Name: "eth2"},
		&link.Link{Name: "enp5"},
	}

	var tests = []struct {
		name     string
		cfg      config.AutoDetectionConfig
		expected config.Ifaces
	}{
		{
			name: "autodetection enabled with direct exclusions",
			cfg: config.AutoDetectionConfig{
				Enabled: true,
				Exclude: []config.IfaceName{"eth2", "eth0", "enp5"},
			},
			expected: config.Ifaces{"eth1": defaultCfg},
		},
		{
			name: "autodetection enabled with regex exclusions",
			cfg: config.AutoDetectionConfig{
				Enabled: true,
				Exclude: []config.IfaceName{"/^eth[02]$/", "/^enp5$/"},
			},
			expected: config.Ifaces{"eth1": defaultCfg},
		},
		{
			name: "autodetection enabled with mixed exclusions",
			cfg: config.AutoDetectionConfig{
				Enabled: true,
				Exclude: []config.IfaceName{"eth0", "/^eth2$/", "enp5"},
			},
			expected: config.Ifaces{"eth1": defaultCfg},
		},
		{
			name: "autodetection enabled with no exclusions",
			cfg: config.AutoDetectionConfig{
				Enabled: true,
				Exclude: []config.IfaceName{},
			},
			expected: config.Ifaces{
				"eth0": defaultCfg,
				"eth1": defaultCfg,
				"eth2": defaultCfg,
				"enp5": defaultCfg,
			},
		},
		{
			name: "autodetection enabled with wildcard regex exclusion",
			cfg: config.AutoDetectionConfig{
				Enabled: true,
				Exclude: []config.IfaceName{"/^eth[0-9]+$/"},
			},
			expected: config.Ifaces{"enp5": defaultCfg},
		},
		{
			name: "autodetection disabled",
			cfg: config.AutoDetectionConfig{
				Enabled: false,
				Exclude: []config.IfaceName{"eth0"},
			},
			expected: config.Ifaces{},
		},
		{
			name: "autodetection enabled - config validation",
			cfg: config.AutoDetectionConfig{
				Enabled: true,
				Exclude: []config.IfaceName{"eth0", "eth2"},
			},
			expected: config.Ifaces{
				"eth1": {
					RingBuffer: config.DefaultCaptureConfig().RingBuffer,
				},
				"enp5": {
					RingBuffer: config.DefaultCaptureConfig().RingBuffer,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detected, err := cm.autodetectIfaces(stubLinks, test.cfg)
			require.NoError(t, err)
			assert.Equal(t, test.expected, detected)
		})
	}
}

func TestFilterMatchingIfaces(t *testing.T) {
	cm := &Manager{}

	directCfg := config.CaptureConfig{RingBuffer: &config.RingBufferConfig{BlockSize: 2048, NumBlocks: 4}}
	regexCfg := config.CaptureConfig{RingBuffer: &config.RingBufferConfig{BlockSize: 4096, NumBlocks: 8}}

	ifaces := config.Ifaces{
		"eth0":        directCfg,
		"/enp[0-9]+/": regexCfg,
	}

	matcher, _, err := ifaces.Matcher()
	require.NoError(t, err)

	stubLinks := link.Links{
		&link.Link{Name: "eth0"},
		&link.Link{Name: "enp3"},
		&link.Link{Name: "enp4"},
		&link.Link{Name: "enp42"},
		&link.Link{Name: "lo"},
	}

	originalHostLinks := hostLinks
	hostLinks = func(...string) (link.Links, error) {
		return stubLinks, nil
	}
	t.Cleanup(func() { hostLinks = originalHostLinks })

	matched, err := cm.filterMatchingIfaces(stubLinks, matcher)
	require.NoError(t, err)

	expected := config.Ifaces{
		"eth0":  directCfg,
		"enp3":  regexCfg,
		"enp4":  regexCfg,
		"enp42": regexCfg,
	}

	assert.Equal(t, expected, matched)
}

func TestUpdateHostLinksUsage(t *testing.T) {
	ctx := context.Background()
	cm := NewManager(noopWriteoutHandler{})
	cm.sourceInitFn = func(*Capture) (Source, error) {
		return nil, fmt.Errorf("expected source init error in test")
	}

	stubLinks := link.Links{
		&link.Link{Name: "eth0"},
		&link.Link{Name: "eth1"},
	}

	defaultCfg := config.DefaultCaptureConfig()

	tests := []struct {
		name                  string
		cfg                   *config.Config
		expectedHostLinkCalls int
	}{
		{
			name: "autodetection disabled without regex",
			cfg: &config.Config{
				Interfaces: config.Ifaces{
					"eth0": defaultCfg,
				},
			},
			expectedHostLinkCalls: 0,
		},
		{
			name: "autodetection disabled with regex",
			cfg: &config.Config{
				Interfaces: config.Ifaces{
					"/^eth[0-9]+$/": defaultCfg,
				},
			},
			expectedHostLinkCalls: 1,
		},
		{
			name: "autodetection enabled",
			cfg: &config.Config{
				AutoDetection: config.AutoDetectionConfig{Enabled: true},
			},
			expectedHostLinkCalls: 1,
		},
	}

	originalHostLinks := hostLinks
	t.Cleanup(func() { hostLinks = originalHostLinks })

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			hostLinks = func(...string) (link.Links, error) {
				calls++
				return stubLinks, nil
			}

			_, _, _, err := cm.Update(ctx, test.cfg)
			require.NoError(t, err)

			assert.Equal(t, test.expectedHostLinkCalls, calls)
		})
	}
}

func TestUpdateHostLinksErrorSurface(t *testing.T) {
	ctx := context.Background()
	cm := &Manager{}

	originalHostLinks := hostLinks
	t.Cleanup(func() { hostLinks = originalHostLinks })

	hostLinks = func(...string) (link.Links, error) {
		return nil, fmt.Errorf("boom")
	}

	defaultCfg := config.DefaultCaptureConfig()

	tests := []struct {
		name string
		cfg  *config.Config
	}{
		{
			name: "regex branch",
			cfg: &config.Config{
				Interfaces: config.Ifaces{
					"/^eth[0-9]+$/": defaultCfg,
				},
			},
		},
		{
			name: "autodetection branch",
			cfg: &config.Config{
				AutoDetection: config.AutoDetectionConfig{Enabled: true},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, _, _, err := cm.Update(ctx, test.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to get host links")
			assert.Contains(t, err.Error(), "boom")
		})
	}
}
