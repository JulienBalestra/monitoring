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
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	client *http.Client
}

func NewPrometheusExporter(conf *collector.Config) collector.Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		client: &http.Client{Timeout: conf.CollectInterval},
	}
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
	return map[string]string{}
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

func (c *Collector) getMetricsFamily(req *http.Request) ([]*dto.MetricFamily, error) {
	var families []*dto.MetricFamily
	resp, err := c.client.Do(req)
	if err != nil {
		return families, fmt.Errorf("request for URL failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return families, fmt.Errorf("request for URL %s returned HTTP status %s", req.URL.String(), resp.Status)
	}
	mediaType, params, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return families, err
	}
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
			newName, ok := c.conf.Options[*mf.Name]
			if !ok {
				continue
			}
			if newName == "" {
				zap.L().Warn("unset metric rename", zap.String("metric", *mf.Name))
				continue
			}
			mf.Name = &newName
			families = append(families, mf)
		}
		return families, nil
	}
	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	if err != nil {
		return families, fmt.Errorf("reading text format failed: %v", err)
	}
	for k, v := range c.conf.Options {
		if k == OptionURL {
			continue
		}
		if v == "" {
			zap.L().Warn("unset metric rename", zap.String("metric", k))
			continue
		}
		mf, ok := metricFamilies[k]
		if !ok {
			continue
		}
		mf.Name = &v
		families = append(families, mf)
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
		}
	}
	return nil
}
