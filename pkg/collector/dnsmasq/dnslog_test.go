package dnsmasq

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCollect(t *testing.T) {
	b, err := ioutil.ReadFile("fixtures/dnsmasq.log")
	require.NoError(t, err)
	queries := make(map[string]*dnsQuery)

	c := newLog(&collector.Config{
		Host:            "entity",
		Tagger:          tagger.NewTagger(),
		CollectInterval: time.Second,
		SeriesCh:        make(chan metrics.Series, 10),
	})
	c.ignoreDomains = make(map[string]struct{})

	c.conf.Tagger.Add("192.168.1.1", tagger.NewTagUnsafe("lease", "host-a"))
	c.conf.Tagger.Add("192.168.1.114", tagger.NewTagUnsafe("lease", "host-b"))
	dt, err := time.Parse(dnsmasqDateFormat, "2020Apr 20 20:30:11")
	require.NoError(t, err)
	c.startTailing = dt

	lines := bytes.Split(b, []byte{'\n'})
	for _, line := range lines {
		c.processLine(queries, line)
	}
	assert.Len(t, queries, 8)
	assert.Equal(t, 5., queries["Adatadoghq.com192.168.1.1"].count)
	assert.Equal(t, 1., queries["AAAAdatadoghq.com192.168.1.1"].count)
	assert.Equal(t, 1., queries["Adatadoghq.com192.168.1.114"].count)
	assert.Equal(t, 1., queries["TXThits.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["TXTmisses.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["TXTevictions.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["TXTcachesize.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["Aa.b1.1.1.1"].count)
	for _, query := range queries {
		require.NoError(t, c.measures.Count(c.queryToSample(query)), query)
		assert.Len(t, c.conf.SeriesCh, 0)
		for i := 0; i < len(c.conf.SeriesCh); i++ {
			t.Errorf("incorrect number of elt in the SeriesCh: %v", <-c.conf.SeriesCh)
		}
	}
	for _, line := range lines {
		c.processLine(queries, line)
	}

	dt, err = time.Parse(dnsmasqDateFormat, "2020Apr 20 21:30:11")
	require.NoError(t, err)
	c.startTailing = dt

	queries = make(map[string]*dnsQuery)
	for _, line := range lines {
		c.processLine(queries, line)
	}

	require.NoError(t, err)
	assert.Equal(t, 3., queries["Adatadoghq.com192.168.1.1"].count)
	assert.Equal(t, 1., queries["AAAAdatadoghq.com192.168.1.1"].count)
	assert.Equal(t, 1., queries["Adatadoghq.com192.168.1.114"].count)
	assert.Equal(t, 1., queries["TXThits.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["TXTmisses.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["TXTevictions.bind127.0.0.1"].count)
	assert.Equal(t, 1., queries["TXTcachesize.bind127.0.0.1"].count)
}
