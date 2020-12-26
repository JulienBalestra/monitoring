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

	optionURL = "exporter-url"

	appProtobuf  = "application/vnd.google.protobuf"
	protoMF      = "io.prometheus.client.MetricFamily"
	acceptHeader = appProtobuf + ";" +
		"proto=" + protoMF + ";" +
		"encoding=delimited;" +
		"q=0.7,text/plain;" +
		"version=0.0.4;" +
		"q=0.3"
)

type Exporter struct {
	conf     *collector.Config
	measures *metrics.Measures

	client *http.Client
}

func NewPrometheusExporter(conf *collector.Config) collector.Collector {
	return &Exporter{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		client: &http.Client{Timeout: conf.CollectInterval},
	}
}

func (c *Exporter) DefaultOptions() map[string]string {
	return map[string]string{}
}

func (c *Exporter) DefaultCollectInterval() time.Duration {
	return time.Second * 30
}

func (c *Exporter) Config() *collector.Config {
	return c.conf
}

func (c *Exporter) IsDaemon() bool { return false }

func (c *Exporter) Name() string {
	return CollectorName
}

func (c *Exporter) getMetricsFamily(req *http.Request) ([]*dto.MetricFamily, error) {
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
		if k == optionURL {
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
		families = append(families, mf)
	}
	return families, nil
}

func (c *Exporter) getTagsFromLabels(tags []string, labels []*dto.LabelPair) []string {
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

func (c *Exporter) Collect(ctx context.Context) error {
	u, ok := c.conf.Options[optionURL]
	if !ok {
		zap.L().Error("missing option", zap.String("options", optionURL))
		return errors.New("missing option " + optionURL)
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
	// TODO dry this
	for _, mf := range families {
		switch *mf.Type {
		case dto.MetricType_COUNTER:
			if len(mf.Metric) != 1 {
				continue
			}
			m := *mf.Metric[0]
			tags := c.conf.Tagger.GetUnstable(c.conf.Host)
			tags = c.getTagsFromLabels(tags, m.Label)
			_ = c.measures.CountWithNegativeReset(&metrics.Sample{
				Name:      c.conf.Options[*mf.Name],
				Value:     *m.Counter.Value,
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			})
		case dto.MetricType_GAUGE:
			if len(mf.Metric) != 1 {
				continue
			}
			m := *mf.Metric[0]
			tags := c.conf.Tagger.GetUnstable(c.conf.Host)
			tags = c.getTagsFromLabels(tags, m.Label)
			c.measures.GaugeDeviation(&metrics.Sample{
				Name:      c.conf.Options[*mf.Name],
				Value:     *m.Gauge.Value,
				Timestamp: now,
				Host:      c.conf.Host,
				Tags:      tags,
			}, c.conf.CollectInterval*3)
		}
	}
	return nil
}
