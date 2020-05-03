package network

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
)

func TestNewConntrack(t *testing.T) {
	c := newConntrack(&collector.Config{
		SeriesCh: make(chan metrics.Series, 50),
		Tagger:   tagger.NewTagger(),
		Host:     "host-a",
	})
	b, err := ioutil.ReadFile("fixtures/conntrack.txt")
	require.NoError(t, err)
	lines := bytes.Split(b, []byte{'\n'})

	stats := make(map[string]*conntrackRecord)
	for _, line := range lines {
		err = c.parseFields(stats, line)
		require.NoError(t, err, string(line))
	}
	// UDP replied
	key := "udp53replied192.168.1.147"
	assert.Equal(t, 6., stats[key].sPackets)
	assert.Equal(t, 362., stats[key].sBytes)

	assert.Equal(t, 6., stats[key].dPackets)
	assert.Equal(t, 626., stats[key].dBytes)

	assert.Equal(t, "53", stats[key].destinationPortRange)
	assert.Equal(t, "replied", stats[key].state)
	assert.Equal(t, "udp", stats[key].protocol)

	// UDP [UNREPLIED]
	key = "udp1024-8191unreplied192.168.1.135"
	assert.Equal(t, 1657., stats[key].sPackets)
	assert.Equal(t, 336371., stats[key].sBytes)

	assert.Equal(t, 0., stats[key].dPackets)
	assert.Equal(t, 0., stats[key].dBytes)

	assert.Equal(t, "1024-8191", stats[key].destinationPortRange)
	assert.Equal(t, "unreplied", stats[key].state)
	assert.Equal(t, "udp", stats[key].protocol)

	// TCP
	key = "tcp443ESTABLISHED192.168.1.101"
	assert.Equal(t, 38., stats[key].sPackets)
	assert.Equal(t, 5399., stats[key].sBytes)

	assert.Equal(t, 42., stats[key].dPackets)
	assert.Equal(t, 11558., stats[key].dBytes)

	assert.Equal(t, "443", stats[key].destinationPortRange)
	assert.Equal(t, "ESTABLISHED", stats[key].state)
	assert.Equal(t, "tcp", stats[key].protocol)

	// ICMP
	key = "icmp-18192.168.1.134"
	assert.Equal(t, 2., stats[key].sPackets)
	assert.Equal(t, 168., stats[key].sBytes)

	assert.Equal(t, 2., stats[key].dPackets)
	assert.Equal(t, 168., stats[key].dBytes)

	assert.Equal(t, "-1", stats[key].destinationPortRange)
	assert.Equal(t, "8", stats[key].state)
	assert.Equal(t, "icmp", stats[key].protocol)
}
