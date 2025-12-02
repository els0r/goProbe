package capture

import (
	"testing"

	"github.com/els0r/goProbe/v4/cmd/goProbe/config"
	"github.com/fako1024/gotools/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutodetectIfaces(t *testing.T) {
	cm := &Manager{
		autoDetectionEnabled: true,
		autoDetectionExclusionSet: map[string]struct{}{
			"eth2": {},
			"eth0": {},
			"enp5": {},
		},
	}

	defaultCfg := config.DefaultCaptureConfig()

	stubLinks := link.Links{
		&link.Link{Name: "eth0"},
		&link.Link{Name: "eth1"},
		&link.Link{Name: "eth2"},
		&link.Link{Name: "enp5"},
	}

	originalHostLinks := hostLinks
	hostLinks = func(...string) (link.Links, error) {
		return stubLinks, nil
	}
	t.Cleanup(func() { hostLinks = originalHostLinks })

	detected, err := cm.autodetectIfaces(stubLinks, defaultCfg)
	require.NoError(t, err)

	expected := config.Ifaces{"eth1": defaultCfg}
	assert.Equal(t, expected, detected)
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
