package dnsmasq

import (
	"bytes"
	"io/ioutil"
	"testing"
	"time"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/metrics"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCollect(t *testing.T) {
	b, err := ioutil.ReadFile("fixtures/dnsmasq.log")
	require.NoError(t, err)
	samples := make(map[string]*metrics.Sample)

	c := newLog(&collector.Config{
		Host:            "entity",
		Tagger:          tagger.NewTagger(),
		CollectInterval: time.Second,
		SeriesCh:        make(chan metrics.Series, 10),
	})

	c.conf.Tagger.Add("192.168.1.1", tagger.NewTagUnsafe("lease", "host-a"))
	c.conf.Tagger.Add("192.168.1.114", tagger.NewTagUnsafe("lease", "host-b"))
	dt, err := time.Parse(dnsmasqDateFormat, "2020Apr 20 20:30:11")
	require.NoError(t, err)
	c.startTailing = dt

	lines := bytes.Split(b, []byte{'\n'})
	for _, line := range lines {
		c.processLine(samples, line)
	}
	assert.Len(t, samples, 7)
	assert.Equal(t, 5., samples["Adatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., samples["AAAAdatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., samples["Adatadoghq.com192.168.1.114"].Value)
	assert.Equal(t, 1., samples["TXThits.bind127.0.0.1"].Value)
	assert.Equal(t, 1., samples["TXTmisses.bind127.0.0.1"].Value)
	assert.Equal(t, 1., samples["TXTevictions.bind127.0.0.1"].Value)
	assert.Equal(t, 1., samples["TXTcachesize.bind127.0.0.1"].Value)
	for _, s := range samples {
		require.NoError(t, c.measures.Count(s), s)
		assert.Len(t, c.conf.SeriesCh, 0)
		for i := 0; i < len(c.conf.SeriesCh); i++ {
			t.Errorf("incorrect number of elt in the SeriesCh: %v", <-c.conf.SeriesCh)
		}
	}
	for _, line := range lines {
		c.processLine(samples, line)
	}

	dt, err = time.Parse(dnsmasqDateFormat, "2020Apr 20 21:30:11")
	require.NoError(t, err)
	c.startTailing = dt

	samples = make(map[string]*metrics.Sample)
	for _, line := range lines {
		c.processLine(samples, line)
	}

	require.NoError(t, err)
	assert.Equal(t, 3., samples["Adatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., samples["AAAAdatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., samples["Adatadoghq.com192.168.1.114"].Value)
	assert.Equal(t, 1., samples["TXThits.bind127.0.0.1"].Value)
	assert.Equal(t, 1., samples["TXTmisses.bind127.0.0.1"].Value)
	assert.Equal(t, 1., samples["TXTevictions.bind127.0.0.1"].Value)
	assert.Equal(t, 1., samples["TXTcachesize.bind127.0.0.1"].Value)
}
