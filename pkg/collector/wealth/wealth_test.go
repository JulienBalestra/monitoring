package wealth

import (
	"context"
	"testing"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/stretchr/testify/require"
)

func TestNewWealth(t *testing.T) {
	c := newWealth(&collector.Config{
		Host:            "entity",
		Tagger:          tagger.NewTagger(),
		CollectInterval: time.Second,
		SeriesCh:        make(chan metrics.Series, 1000),
	})
	c.ddogStockFile = "fixtures/ddog.json"
	err := c.Collect(context.TODO())
	require.NoError(t, err)
}
