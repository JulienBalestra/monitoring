package dnsmasq

import (
	"bytes"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	"github.com/JulienBalestra/metrics/pkg/datadog"

	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogCollect(t *testing.T) {
	b, err := ioutil.ReadFile(path.Join(os.Getenv("GOPATH"), "src/github.com/JulienBalestra/metrics/pkg/collector/dnsmasq/fixtures/dnsmasq.log"))
	require.NoError(t, err)
	counters := make(datadog.Counter)

	c := newLog(&collector.Config{
		Host:            "entity",
		Tagger:          tagger.NewTagger(),
		CollectInterval: time.Second,
	})

	dt, err := time.Parse(dnsmasqDateFormat, "2020Apr 20 20:30:11")
	require.NoError(t, err)
	c.startTailing = dt

	lines := bytes.Split(b, []byte{'\n'})
	for _, line := range lines {
		c.processLine(counters, line)
	}
	assert.Equal(t, 5., counters["Adatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., counters["AAAAdatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., counters["Adatadoghq.com192.168.1.114"].Value)
	assert.Equal(t, 1., counters["TXThits.bind127.0.0.1"].Value)
	assert.Equal(t, 1., counters["TXTmisses.bind127.0.0.1"].Value)
	assert.Equal(t, 1., counters["TXTevictions.bind127.0.0.1"].Value)
	assert.Equal(t, 1., counters["TXTcachesize.bind127.0.0.1"].Value)

	dt, err = time.Parse(dnsmasqDateFormat, "2020Apr 20 21:30:11")
	require.NoError(t, err)
	c.startTailing = dt

	counters = make(datadog.Counter)
	for _, line := range lines {
		c.processLine(counters, line)
	}

	require.NoError(t, err)
	assert.Equal(t, 3., counters["Adatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., counters["AAAAdatadoghq.com192.168.1.1"].Value)
	assert.Equal(t, 1., counters["Adatadoghq.com192.168.1.114"].Value)
	assert.Equal(t, 1., counters["TXThits.bind127.0.0.1"].Value)
	assert.Equal(t, 1., counters["TXTmisses.bind127.0.0.1"].Value)
	assert.Equal(t, 1., counters["TXTevictions.bind127.0.0.1"].Value)
	assert.Equal(t, 1., counters["TXTcachesize.bind127.0.0.1"].Value)
}
