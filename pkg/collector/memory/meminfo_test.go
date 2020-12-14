package memory

import (
	"context"
	"testing"

	"github.com/JulienBalestra/monitoring/pkg/datadog"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemory(t *testing.T) {
	m := newMemory(&collector.Config{
		MetricsClient: datadog.NewClient(&datadog.Config{
			ChanSize: 1000,
		}),
		Tagger: tagger.NewTagger(),
		Options: map[string]string{
			optionMemInfoFile: "fixtures/dd-wrt.meminfo",
		},
	})
	err := m.Collect(context.TODO())
	require.NoError(t, err)
	assert.Len(t, m.conf.MetricsClient.ChanSeries, len(m.mapping))

	m = newMemory(&collector.Config{
		MetricsClient: datadog.NewClient(&datadog.Config{
			ChanSize: 1000,
		}),
		Tagger: tagger.NewTagger(),
		Options: map[string]string{
			optionMemInfoFile: "fixtures/pi.meminfo",
		},
	})
	err = m.Collect(context.TODO())
	require.NoError(t, err)
	assert.Len(t, m.conf.MetricsClient.ChanSeries, len(m.mapping)-1)
}
