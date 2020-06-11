package wealth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
)

const (
	CollectorWealthName = "wealth"
)

type Wealth struct {
	conf     *collector.Config
	measures *metrics.Measures

	ddogStockFile string
	client        *http.Client
}

func NewWealth(conf *collector.Config) collector.Collector {
	return newWealth(conf)
}

func newWealth(conf *collector.Config) *Wealth {
	return &Wealth{
		conf:          conf,
		measures:      metrics.NewMeasures(conf.SeriesCh),
		ddogStockFile: os.Getenv("DDOG_STOCK_FILE"),
		client:        &http.Client{},
	}
}

func (c *Wealth) Config() *collector.Config {
	return c.conf
}

func (c *Wealth) IsDaemon() bool { return false }

func (c *Wealth) Name() string {
	return CollectorWealthName
}

func (c *Wealth) Collect(ctx context.Context) error {
	zctx := zap.L().With(
		zap.String("ddogStockFile", c.ddogStockFile),
	)
	now := time.Now()
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://query1.finance.yahoo.com/v7/finance/chart/DDOG?&interval=1d",
		nil,
	)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		zctx.Error("failed to query yahoo chart", zap.Error(err))
		return err
	}
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		zctx.Error("failed to read yahoo response", zap.Error(err))
		return err
	}
	err = resp.Body.Close()
	if err != nil {
		zctx.Warn("failed to close body response", zap.Error(err))
	}
	q := &Quote{}
	err = json.Unmarshal(bodyBytes, q)
	if err != nil {
		return err
	}
	if len(q.Chart.Result) != 1 {
		return fmt.Errorf("failed to parse yahoo chart: %v", string(bodyBytes))
	}
	b, err := ioutil.ReadFile(c.ddogStockFile)
	if err != nil {
		zctx.Error("failed to read stock file", zap.Error(err))
		return err
	}
	var options []Option
	err = json.Unmarshal(b, &options)
	if err != nil {
		zctx.Error("failed to unmarshal yahoo response", zap.ByteString("body", bodyBytes))
		return err
	}
	ddog := q.Chart.Result[0].Meta.RegularMarketPrice
	zctx = zctx.With(
		zap.Float64("ddog", ddog),
	)
	c.measures.Gauge(&metrics.Sample{
		Name:      "stock.value",
		Value:     ddog,
		Timestamp: now,
		Host:      c.conf.Host,
		Tags: []string{
			"stock:ddog",
		},
	})
	for _, elt := range options {
		zctx = zctx.With(
			zap.Float64("exerciseTaxable", elt.GetExerciseTaxable()),
			zap.Float64("exerciseTaxes", elt.GetExerciseTaxes()),
			zap.Float64("gainTaxable", elt.GetGainTaxable(ddog)),
			zap.Float64("gainTaxes", elt.GetGainTaxes(ddog)),
			zap.Float64("inThePocket", elt.InThePocket(ddog)),
			zap.Float64("marketValue", elt.GetMarketValue(ddog)),
		)
		zctx.Debug("processing option")
		c.measures.Gauge(&metrics.Sample{
			Name:      "stock.taxes",
			Value:     elt.GetExerciseTaxes(),
			Timestamp: now,
			Host:      c.conf.Host,
			Tags: []string{
				"stock:ddog",
				"holding:options",
				"tax:exercise",
				"date:" + elt.ExerciseDate,
			},
		})

		c.measures.Gauge(&metrics.Sample{
			Name:      "stock.taxes",
			Value:     elt.GetGainTaxes(ddog),
			Timestamp: now,
			Host:      c.conf.Host,
			Tags: []string{
				"stock:ddog",
				"holding:options",
				"tax:gain",
				"date:" + elt.ExerciseDate,
			},
		})

		c.measures.Gauge(&metrics.Sample{
			Name:      "stock.holding",
			Value:     elt.GetMarketValue(ddog),
			Timestamp: now,
			Host:      c.conf.Host,
			Tags: []string{
				"stock:ddog",
				"holding:options",
				"date:" + elt.ExerciseDate,
			},
		})
	}

	return nil
}
