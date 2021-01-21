package dnslogs

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/collectors/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

const (
	CollectorName = "dnsmasq-log"

	dnsmasqDateFormat  = "2006Jan 2 15:04:05"
	dnsmasqQueryMetric = "dnsmasq.dns.query"

	optionLogFacilityKey = "log-facility-file"
)

type Collector struct {
	conf     *collector.Config
	measures *metrics.Measures

	firstSep, secondSep, thirdSep []byte
	startTailing                  time.Time
	leaseTag                      *tagger.Tag

	ignoreDomains map[string]struct{}
}

type dnsQuery struct {
	queryType string
	domain    string
	ipAddress string
	count     float64
}

func newLog(conf *collector.Config) *Collector {
	return &Collector{
		conf:     conf,
		measures: metrics.NewMeasures(conf.MetricsClient.ChanSeries),

		firstSep:  []byte("]: query["),
		secondSep: []byte("] "),
		thirdSep:  []byte{' '},
		leaseTag:  tagger.NewTagUnsafe(exported.LeaseKey, tagger.MissingTagValue),

		// these domains are ignored and not submitted as metrics
		ignoreDomains: map[string]struct{}{
			exported.HitsQueryBind:       {},
			exported.MissesQueryBind:     {},
			exported.InsertionsQueryBind: {},
			exported.EvictionsQueryBind:  {},
			exported.CachesizeQueryBind:  {},
		},
	}
}

func NewDnsMasqLog(conf *collector.Config) collector.Collector {
	return collector.WithDefaults(newLog(conf))
}

func (c *Collector) SubmittedSeries() float64 {
	return c.measures.GetTotalSubmittedSeries()
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
		/*
			dnsmasq configfile:
			log-queries
			log-facility=/tmp/dnsmasq.log
		*/
		optionLogFacilityKey: "/tmp/dnsmasq.log",
	}
}

func (c *Collector) DefaultCollectInterval() time.Duration {
	return time.Second * 10
}

func (c *Collector) IsDaemon() bool { return true }

func (c *Collector) Config() *collector.Config {
	return c.conf
}

func (c *Collector) Name() string {
	return CollectorName
}

func (c *Collector) queryToSample(query *dnsQuery) *metrics.Sample {
	tags := append(c.conf.Tagger.GetUnstableWithDefault(query.ipAddress, c.leaseTag),
		c.Tags()...)
	tags = append(tags, "domain:"+query.domain, "type:"+query.queryType)
	return &metrics.Sample{
		Name:  dnsmasqQueryMetric,
		Value: query.count,
		Time:  time.Now(),
		Host:  c.conf.Host,
		Tags:  tags,
	}
}

func (c *Collector) Collect(ctx context.Context) error {
	lineCh := make(chan []byte)
	defer close(lineCh)

	firstStart := true
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			default:
				dnsmasqLogQueriesFile, ok := c.conf.Options[optionLogFacilityKey]
				if !ok {
					zap.L().Error("missing option", zap.String("options", optionLogFacilityKey))
					wait, cancel := context.WithTimeout(ctx, time.Second*60)
					<-wait.Done()
					cancel()
					continue
				}
				err := c.tail(ctx, lineCh, dnsmasqLogQueriesFile, firstStart)
				if err != nil {
					zap.L().Error("failed tailing", zap.Error(err))
					wait, cancel := context.WithTimeout(ctx, time.Second)
					<-wait.Done()
					cancel()
				}
				firstStart = false
			}
		}
	}()

	// 127.0.0.1 can do dns query
	c.conf.Tagger.Update("127.0.0.1", tagger.NewTagUnsafe(exported.LeaseKey, "localhost"))
	c.startTailing = time.Now()
	queries := make(map[string]*dnsQuery)

	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	zctx := zap.L().With(
		zap.String("collection", c.Name()),
	)
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			zctx.Info("end of collection")
			return nil

		case <-ticker.C:
			if len(queries) == 0 {
				continue
			}
			var err error
			for _, query := range queries {
				err = c.measures.Incr(c.queryToSample(query))
				// TODO self telemetry
				if err == nil {
					continue
				}
				zctx.Error("failed to run collection", zap.Error(err))
			}
			queries = make(map[string]*dnsQuery)
			c.measures.Purge()
			if err != nil {
				continue
			}
			zctx.Info("ok")

		case line := <-lineCh:
			c.processLine(queries, line)
		}
	}
}

func (c *Collector) tail(ctx context.Context, ch chan []byte, f string, end bool) error {
	file, err := os.Open(f)
	if err != nil {
		return err
	}
	defer file.Close()

	if end {
		_, err = file.Seek(0, 2)
		if err != nil {
			return err
		}
	}

	// TODO fix this reader
	reader := bufio.NewReaderSize(file, 1024*6)
	var prevSize int64
	stalingFileCount := 0
	for {
		select {
		case <-ctx.Done():
			return nil

		default:
			// TODO improve this reader
			line, err := reader.ReadBytes('\n')
			if err == nil {
				ch <- line
				continue
			}
			if err != io.EOF {
				return err
			}
			s, err := file.Stat()
			if err != nil {
				return err
			}
			if s.Size() < prevSize {
				zap.L().Info("truncated file",
					zap.String("file", f),
					zap.Int64("currentSize", s.Size()),
					zap.Int64("previousSize", prevSize),
					zap.Int("stalingFileCount", stalingFileCount),
				)
				return nil
			}
			wait, cancel := context.WithTimeout(ctx, time.Second)
			<-wait.Done()
			cancel()
			if s.Size() > prevSize {
				prevSize = s.Size()
				stalingFileCount = 0
				continue
			}
			if stalingFileCount < 30 {
				stalingFileCount++
				continue
			}
			zap.L().Info("staling file",
				zap.String("file", f),
				zap.Int64("currentSize", s.Size()),
				zap.Int64("previousSize", prevSize),
				zap.Int("stalingFileCount", stalingFileCount),
			)
			return nil
		}
	}
}

func (c *Collector) processLine(counters map[string]*dnsQuery, line []byte) {
	// minimal len of a dnsquery
	if len(line) < 53 {
		return
	}
	// Apr 20 21:35:07
	// 2006Apr 20 21:35:07
	t, err := time.Parse(dnsmasqDateFormat, c.startTailing.Format("2006")+string(line[:15]))
	if err != nil {
		zap.L().Error("failed to parse date in line", zap.Error(err), zap.ByteString("line", line))
		return
	}
	if !c.startTailing.Before(t) {
		return
	}
	beginQueryType := bytes.Index(line, c.firstSep)
	if beginQueryType == -1 {
		return
	}
	beginQueryType += len(c.firstSep)
	if len(line) < beginQueryType {
		return
	}
	endQueryType := bytes.Index(line, c.secondSep)
	if beginQueryType == -1 {
		return
	}
	queryType := string(line[beginQueryType:endQueryType])
	if len(line) < beginQueryType {
		return
	}
	endQueryType += 2
	// api.datadoghq.com from 192.168.1.1
	//                  ^
	endOfquery := bytes.Index(line[endQueryType:], c.thirdSep)
	if endOfquery == -1 {
		return
	}
	endOfquery += endQueryType
	// api.datadoghq.com
	//				^
	lastQueryDot := bytes.LastIndexByte(line[endQueryType:endOfquery], '.')
	if lastQueryDot == -1 {
		return
	}
	lastQueryDot += endQueryType
	// api.datadoghq.com
	//	  ^ ? return -1 so adding 1 == 0
	eventualDot := bytes.LastIndexByte(line[endQueryType:lastQueryDot], '.')
	domain := string(line[endQueryType+eventualDot+1 : endOfquery])
	_, ok := c.ignoreDomains[domain]
	if ok {
		return
	}
	// from 192.168.1.1\n
	//     ^
	ipAddress := strings.TrimRight(string(line[endOfquery+6:]), "\n")
	key := queryType + domain + ipAddress
	m, ok := counters[key]
	if ok {
		m.count++
		return
	}
	counters[key] = &dnsQuery{
		queryType: queryType,
		domain:    domain,
		ipAddress: ipAddress,
		count:     1,
	}
}
