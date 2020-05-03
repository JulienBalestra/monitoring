package main

import (
	"context"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

func main() {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*2)
	defer cancel()

	c := datadog.NewClient(&datadog.Config{
		Host:          "my-host",
		DatadogAPIKey: "fake-api-key********************",
		SendInterval:  time.Second * 60,
	})

	series := []metrics.Series{
		{
			Metric: "custom.metrics",
			Points: [][]float64{
				{
					float64(time.Now().Add(-time.Second * 30).Unix()),
					1,
				},
				{
					float64(time.Now().Unix()),
					2,
				},
			},
			Type: metrics.TypeGauge,
			// take leverage of the tagger to manage tags
			// import "github.com/JulienBalestra/monitoring/pkg/tagger"
			Host: "my-host",
			Tags: []string{"role:web", "tier:frontend"},
		},
	}

	// synchronously
	_ = c.SendSeries(ctx, series)

	// in background
	waitGroup := sync.WaitGroup{}
	defer waitGroup.Wait()
	waitGroup.Add(1)
	go func() {
		c.Run(ctx)
		waitGroup.Done()
	}()

	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, s := range series {
				// client is going to aggregate the same metrics before sending them
				c.ChanSeries <- s
			}
		}
	}
}
