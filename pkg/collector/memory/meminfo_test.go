package memory

import (
	"testing"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemory(t *testing.T) {
	m := newMemory(&collector.Config{
		SeriesCh: make(chan metrics.Series, 1000),
		Tagger:   tagger.NewTagger(),
	})
	err := m.collect("fixtures/dd-wrt.meminfo")
	require.NoError(t, err)
	assert.Len(t, m.conf.SeriesCh, len(m.mapping))

	m = newMemory(&collector.Config{
		SeriesCh: make(chan metrics.Series, 1000),
		Tagger:   tagger.NewTagger(),
	})
	err = m.collect("fixtures/pi.meminfo")
	require.NoError(t, err)
	assert.Len(t, m.conf.SeriesCh, len(m.mapping)-1)
}
