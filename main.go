package main

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/dnsmasq"
	"github.com/JulienBalestra/metrics/pkg/collector/load"
	"github.com/JulienBalestra/metrics/pkg/collector/memory"
	"github.com/JulienBalestra/metrics/pkg/collector/network"
	"github.com/JulienBalestra/metrics/pkg/collector/temperature"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func notifySignals(ctx context.Context, cancel context.CancelFunc) {
	signals := make(chan os.Signal)
	defer close(signals)
	defer signal.Stop(signals)

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of signal handling")
			return

		case sig := <-signals:
			log.Printf("signal %s received", sig)
			cancel()
		}
	}
}

func main() {
	var err error
	// TODO use a real command line parser:
	host := os.Getenv("HOSTNAME")
	if host == "" {
		log.Fatalf("empty envvar HOSTNAME")
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatalf("empty envvar API_KEY")
	}

	var hostTags []*tagger.Tag
	hostTagsStr := os.Getenv("HOST_TAGS")
	if hostTagsStr != "" {
		hostTags, err = tagger.CreateTags(strings.Split(hostTagsStr, ",")...)
		if err != nil {
			log.Fatalf("cannot parse given HOST_TAGS: %v", err)
		}
		log.Printf("parsed the following host tags: %q -> %s", hostTagsStr, hostTags)
	}

	ctx, cancel := context.WithCancel(context.TODO())
	waitGroup := &sync.WaitGroup{}

	waitGroup.Add(1)
	go func() {
		notifySignals(ctx, cancel)
		waitGroup.Done()
	}()

	tags := tagger.NewTagger()
	tags.Add(host, hostTags...)

	// not really useful but doesn't hurt either
	tags.Print()

	client := datadog.NewClient(host, apiKey, tags)
	waitGroup.Add(1)
	go func() {
		client.Run(ctx)
		waitGroup.Done()
	}()
	// TODO lifecycle of this chan / create outside ? Wrap ?
	defer close(client.ChanSeries)

	collecterConfig := &collector.Config{
		SeriesCh:        client.ChanSeries,
		Tagger:          tags,
		Host:            host,
		CollectInterval: time.Second * 15,
	}

	for _, c := range []collector.Collector{
		load.NewLoadReporter(collecterConfig.
			WithCollectorName("load").
			OverrideCollectInterval(time.Second * 10)),

		network.NewStatisticsReporter(collecterConfig.
			WithCollectorName("network/statistics").
			OverrideCollectInterval(time.Second * 10)),

		network.NewConntrackReporter(collecterConfig.
			WithCollectorName("network/conntrack")),

		network.NewARPReporter(collecterConfig.
			WithCollectorName("network/arp")),

		dnsmasq.NewDnsMasqReporter(collecterConfig.
			WithCollectorName("dnsmasq").
			OverrideCollectInterval(collecterConfig.CollectInterval * 2)),

		temperature.NewTemperatureReporter(collecterConfig.
			WithCollectorName("temperature").
			OverrideCollectInterval(collecterConfig.CollectInterval * 2)),

		memory.NewMemoryReporter(collecterConfig.
			WithCollectorName("meminfo").
			OverrideCollectInterval(collecterConfig.CollectInterval * 2)),
	} {
		select {
		case <-ctx.Done():
			break
		default:
			waitGroup.Add(1)
			go func(coll collector.Collector) {
				collector.RunCollection(ctx, coll)
				waitGroup.Done()
			}(c)
		}
	}
	// TODO: maybe add something else like version
	stableTag := "ts:" + strconv.FormatInt(time.Now().Unix(), 10)
	client.MetricClientUp(stableTag)
	<-ctx.Done()

	ctxShutdown, _ := context.WithTimeout(context.Background(), time.Second*5)
	_ = client.MetricClientShutdown(ctxShutdown, stableTag)
	waitGroup.Wait()
	log.Printf("program exit")
}
