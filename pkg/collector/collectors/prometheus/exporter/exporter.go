package exporter

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/matttproud/golang_protobuf_extensions/pbutil"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"go.uber.org/zap"
)

const (
	CollectorName = "prometheus"

	OptionURL = "exporter-url"

	appProtobuf  = "application/vnd.google.protobuf"
	protoMF      = "io.prometheus.client.MetricFamily"
	acceptHeader = appProtobuf + ";" +
		"proto=" + protoMF + ";" +
		"encoding=delimited;" +
		"q=0.7,text/plain;" +
		"version=0.0.4;" +
		"q=0.3"

	SourceGoRoutinesMetrics      = "go_goroutines"
	DestinationGoroutinesMetrics = "golang.runtime.goroutines"

	SourceGoMemstatsHeapMetrics      = "go_memstats_heap_alloc_bytes"
	DestinationGoMemstatsHeapMetrics = "golang.heap.alloc"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	client        *http.Client
	metricsMap    map[string][]func(*dto.MetricFamily)
	wantedMetrics int
}

func buildRenameFunction(n string) func(family *dto.MetricFamily) {
	return func(family *dto.MetricFamily) {
		family.Name = &n
	}
}

func NewPrometheusExporter(conf *collector.Config) collector.Collector {
	m := make(map[string][]func(*dto.MetricFamily))
	for k, v := range conf.Options {
		if k == OptionURL {
			continue
		}
		if v == "" {
			m[k] = make([]func(family *dto.MetricFamily), 0)
			continue
		}
		m[k] = []func(family *dto.MetricFamily){buildRenameFunction(v)}
	}
	return &Collector{
		conf:          conf,
		measures:      metrics.NewMeasures(conf.MetricsClient.ChanSeries),
		metricsMap:    m,
		wantedMetrics: len(m),
		client:        &http.Client{Timeout: conf.CollectInterval},
	}
}

func (c *Collector) SubmittedSeries() float64 {
	return c.measures.GetTotalSubmittedSeries()
}

func (c *Collector) AddMappingFunction(metricSourceName string, f func(*dto.MetricFamily)) {
	c.metricsMap[metricSourceName] = append(c.metricsMap[metricSourceName], f)
}

func (c *Collector) DefaultTags() []string {
	return []string{
		"collector:" + CollectorName,
	}
}

func (c *Collector) Tags() []string {
	return append(c.conf.Tagger.GetUnstable(c.conf.Host), c.conf.Tags...)
}

func (c *Collector) DefaultOptions() map[string]string {
	return map[string]string{
		SourceGoMemstatsHeapMetrics: DestinationGoMemstatsHeapMetrics,
		SourceGoRoutinesMetrics:     DestinationGoroutinesMetrics,
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) IsDaemon() bool { return false }

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) getMetricsFamily(req *http.Request) (map[string]*dto.MetricFamily, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request for URL failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request for URL %s returned HTTP status %s", req.URL.String(), resp.Status)
	}
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	families := make(map[string]*dto.MetricFamily, c.wantedMetrics)
	if mediaType == appProtobuf && params["encoding"] == "delimited" && params["proto"] == protoMF {
		for {
			mf := &dto.MetricFamily{}
			_, err = pbutil.ReadDelimited(resp.Body, mf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return families, fmt.Errorf("reading metric family protocol buffer failed: %v", err)
			}
			functions, ok := c.metricsMap[*mf.Name]
			if !ok {
				continue
			}
			families[*mf.Name] = mf
			for _, f := range functions {
				f(mf)
			}
			if len(families) == c.wantedMetrics {
				return families, nil
			}
		}
		for k := range c.metricsMap {
			_, ok := families[k]
			if !ok {
				zap.L().Debug("missing metric", zap.String("name", k))
			}
		}
		return families, nil
	}
	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading text format failed: %v", err)
	}
	for name, functions := range c.metricsMap {
		mf, ok := metricFamilies[name]
		if !ok {
			zap.L().Debug("missing metric", zap.String("name", name))
			continue
		}
		families[*mf.Name] = mf
		for _, f := range functions {
			f(mf)
		}
	}
	return families, nil
}

func (c *Collector) getTagsFromLabels(tags []string, labels []*dto.LabelPair) []string {
	for _, elt := range labels {
		if *elt.Value == "" {
			continue
		}
		if *elt.Name == "" {
			continue
		}
		tags = append(tags, *elt.Name+":"+*elt.Value)
	}
	return tags
}

func (c *Collector) Collect(ctx context.Context) error {
	u, ok := c.conf.Options[OptionURL]
	if !ok {
		zap.L().Error("missing option", zap.String("options", OptionURL))
		return errors.New("missing option " + OptionURL)
	}
	ctx, cancel := context.WithTimeout(ctx, c.conf.CollectInterval)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Accept", acceptHeader)
	now := time.Now()
	families, err := c.getMetricsFamily(req)
	if err != nil {
		return err
	}
	tags := c.Tags()
	for _, mf := range families {
		switch *mf.Type {
		case dto.MetricType_COUNTER:
			for _, m := range mf.Metric {
				_ = c.measures.CountWithNegativeReset(&metrics.Sample{
					Name:  *mf.Name,
					Value: *m.Counter.Value,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  c.getTagsFromLabels(tags, m.Label),
				})
			}
		case dto.MetricType_GAUGE:
			for _, m := range mf.Metric {
				c.measures.GaugeDeviation(&metrics.Sample{
					Name:  *mf.Name,
					Value: *m.Gauge.Value,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  c.getTagsFromLabels(tags, m.Label),
				}, c.conf.CollectInterval*c.conf.CollectInterval)
			}

		case dto.MetricType_SUMMARY:
			for _, m := range mf.Metric {
				count, sum := float64(*m.Summary.SampleCount), *m.Summary.SampleSum
				labelsAsTags := c.getTagsFromLabels(tags, m.Label)
				_ = c.measures.CountWithNegativeReset(&metrics.Sample{
					Name:  *mf.Name + ".count",
					Value: count,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  labelsAsTags,
				})
				c.measures.GaugeDeviation(&metrics.Sample{
					Name:  *mf.Name + ".sum",
					Value: sum,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  labelsAsTags,
				}, c.conf.CollectInterval*c.conf.CollectInterval)
				for _, q := range m.Summary.Quantile {
					c.measures.GaugeDeviation(&metrics.Sample{
						Name:  *mf.Name,
						Value: *q.Value,
						Time:  now,
						Host:  c.conf.Host,
						Tags:  append(labelsAsTags, fmt.Sprintf("quantile:%g", *q.Quantile)),
					}, c.conf.CollectInterval*c.conf.CollectInterval)
				}
				if count == 0 || sum == 0 {
					continue
				}
				// avoid NaN value
				c.measures.GaugeDeviation(&metrics.Sample{
					Name:  *mf.Name + ".avg",
					Value: sum / count,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  labelsAsTags,
				}, c.conf.CollectInterval*c.conf.CollectInterval)
			}

		case dto.MetricType_HISTOGRAM:
			for _, m := range mf.Metric {
				count, sum := float64(*m.Histogram.SampleCount), *m.Histogram.SampleSum
				labelsAsTags := c.getTagsFromLabels(tags, m.Label)
				_ = c.measures.CountWithNegativeReset(&metrics.Sample{
					Name:  *mf.Name + ".count",
					Value: count,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  labelsAsTags,
				})
				c.measures.GaugeDeviation(&metrics.Sample{
					Name:  *mf.Name + ".sum",
					Value: sum,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  labelsAsTags,
				}, c.conf.CollectInterval*c.conf.CollectInterval)
				for _, b := range m.Histogram.Bucket {
					c.measures.GaugeDeviation(&metrics.Sample{
						Name:  *mf.Name,
						Value: float64(*b.CumulativeCount),
						Time:  now,
						Host:  c.conf.Host,
						Tags:  append(labelsAsTags, fmt.Sprintf("le:%g", *b.UpperBound)),
					}, c.conf.CollectInterval*c.conf.CollectInterval)
				}
				if count == 0 || sum == 0 {
					continue
				}
				// avoid NaN value
				c.measures.GaugeDeviation(&metrics.Sample{
					Name:  *mf.Name + ".avg",
					Value: sum / count,
					Time:  now,
					Host:  c.conf.Host,
					Tags:  labelsAsTags,
				}, c.conf.CollectInterval*c.conf.CollectInterval)
			}
		}
	}
	return nil
}
