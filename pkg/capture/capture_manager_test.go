package capture

import (
	"testing"

	"github.com/els0r/goProbe/v4/cmd/goProbe/config"
	"github.com/fako1024/gotools/link"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutodetectIfaces(t *testing.T) {
	cm := &Manager{}

	defaultCfg := config.CaptureConfig{
		RingBuffer: &config.RingBufferConfig{BlockSize: 1024, NumBlocks: 2},
	}

	ifaces := config.Ifaces{
		config.InterfaceAuto: defaultCfg,
		"eth0":               {Disable: true},
		"/enp[0-9]+/":        {Disable: true},
	}

	matcher, _, err := ifaces.Matchers()
	require.NoError(t, err)

	stubLinks := link.Links{
		&link.Link{Name: "eth0"},
		&link.Link{Name: "eth1"},
		&link.Link{Name: "enp5"},
	}

	originalHostLinks := hostLinks
	hostLinks = func(...string) (link.Links, error) {
		return stubLinks, nil
	}
	t.Cleanup(func() { hostLinks = originalHostLinks })

	detected, err := cm.autodetectIfaces(matcher, defaultCfg)
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

	matcher, _, err := ifaces.Matchers()
	require.NoError(t, err)

	stubLinks := link.Links{
		&link.Link{Name: "eth0"},
		&link.Link{Name: "enp3"},
		&link.Link{Name: "lo"},
	}

	originalHostLinks := hostLinks
	hostLinks = func(...string) (link.Links, error) {
		return stubLinks, nil
	}
	t.Cleanup(func() { hostLinks = originalHostLinks })

	matched, err := cm.filterMatchingIfaces(matcher)
	require.NoError(t, err)

	expected := config.Ifaces{
		"eth0": directCfg,
		"enp3": regexCfg,
	}

	assert.Equal(t, expected, matched)
}
