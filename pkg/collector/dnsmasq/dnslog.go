package dnsmasq

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/dnsmasq/exported"
	"github.com/JulienBalestra/monitoring/pkg/metrics"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
)

const (
	CollectorDnsMasqLogName = "dnsmasq-log"

	dnsmasqLogPath = "/tmp/dnsmasq.log"

	dnsmasqDateFormat  = "2006Jan 2 15:04:05"
	dnsmasqQueryMetric = "dnsmasq.dns.query"
)

type Log struct {
	conf     *collector.Config
	measures *metrics.Measures

	firstSep, secondSep, thirdSep []byte
	startTailing                  time.Time
	year                          string
	leaseTag                      *tagger.Tag
}

type dnsQuery struct {
	queryType string
	domain    string
	ipAddress string
	count     float64
}

func newLog(conf *collector.Config) *Log {
	return &Log{
		conf:     conf,
		measures: metrics.NewMeasures(conf.SeriesCh),

		firstSep:  []byte("]: query["),
		secondSep: []byte("] "),
		thirdSep:  []byte{' '},
		year:      time.Now().Format("2006"),
		leaseTag:  tagger.NewTagUnsafe(exported.LeaseKey, tagger.MissingTagValue),
	}
}

func NewDnsMasqLog(conf *collector.Config) collector.Collector {
	return newLog(conf)
}

func (c *Log) IsDaemon() bool { return true }

func (c *Log) Config() *collector.Config {
	return c.conf
}

func (c *Log) Name() string {
	return CollectorDnsMasqLogName
}

func (c *Log) queryToSample(query *dnsQuery) *metrics.Sample {
	tags := append(c.conf.Tagger.GetUnstableWithDefault(query.ipAddress, c.leaseTag),
		c.conf.Tagger.GetUnstable(c.conf.Host)...)
	tags = append(tags, "domain:"+query.domain, "type:"+query.queryType)
	return &metrics.Sample{
		Name:      dnsmasqQueryMetric,
		Value:     query.count,
		Timestamp: time.Now(),
		Host:      c.conf.Host,
		Tags:      tags,
	}
}

func (c *Log) Collect(ctx context.Context) error {
	lineCh := make(chan []byte)
	defer close(lineCh)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				wg.Done()
				return
			default:
				err := c.tail(ctx, lineCh, dnsmasqLogPath)
				if err != nil {
					log.Printf("failed tailing: %v", err)
					wait, cancel := context.WithTimeout(ctx, time.Second*5)
					<-wait.Done()
					cancel()
				}
			}
		}
	}()
	c.year = time.Now().Format("2006")
	c.startTailing = time.Now()
	queries := make(map[string]*dnsQuery)

	ticker := time.NewTicker(c.conf.CollectInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			log.Printf("end of collection: %s", c.Name())
			return nil

		case <-ticker.C:
			if len(queries) == 0 {
				continue
			}
			var err error
			for _, query := range queries {
				err = c.measures.Incr(c.queryToSample(query))
				if err == nil {
					continue
				}
				log.Printf("failed to run collection %s: %v", c.Name(), err)
			}
			if err == nil {
				log.Printf("successfully run collection: %s", c.Name())
			}
			queries = make(map[string]*dnsQuery)
			c.measures.Purge()

		case line := <-lineCh:
			c.processLine(queries, line)
		}
	}
}

func (c *Log) tail(ctx context.Context, ch chan []byte, f string) error {
	file, err := os.Open(f)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(0, 2)
	if err != nil {
		return err
	}

	// 127.0.0.1 can do dns query
	c.conf.Tagger.Update("127.0.0.1", tagger.NewTagUnsafe(exported.LeaseKey, "localhost"))

	// TODO fix this reader
	reader := bufio.NewReaderSize(file, 1024*6)
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
			wait, cancel := context.WithTimeout(ctx, time.Second)
			<-wait.Done()
			cancel()
		}
	}
}

func (c *Log) processLine(counters map[string]*dnsQuery, line []byte) {
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
	s := string(line[:15])
	// Apr 20 21:35:07
	// 2006Apr 20 21:35:07
	t, err := time.Parse(dnsmasqDateFormat, c.year+s)
	if err != nil {
		log.Printf("failed to parse date in line: %q %v", string(line), err)
		return
	}
	if !c.startTailing.Before(t) {
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
