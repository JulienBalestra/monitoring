package main

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collecter"
	"github.com/JulienBalestra/metrics/pkg/collecter/dnsmasq"
	"github.com/JulienBalestra/metrics/pkg/collecter/load"
	"github.com/JulienBalestra/metrics/pkg/collecter/memory"
	"github.com/JulienBalestra/metrics/pkg/collecter/network"
	"github.com/JulienBalestra/metrics/pkg/collecter/temperature"
	"github.com/JulienBalestra/metrics/pkg/datadog"
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
	// TODO use a real command line parser:
	host := os.Getenv("HOSTNAME")
	if host == "" {
		log.Fatalf("empty envvar HOSTNAME")
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatalf("empty envvar API_KEY")
	}

	var hostTags []string
	hostTagsStr := os.Getenv("HOST_TAGS")
	if hostTagsStr != "" {
		hostTags = strings.Split(hostTagsStr, ",")
		log.Printf("parsed the following host tags: %q -> %s", hostTagsStr, hostTags)
	}

	ctx, cancel := context.WithCancel(context.TODO())
	waitGroup := &sync.WaitGroup{}

	waitGroup.Add(1)
	go func() {
		notifySignals(ctx, cancel)
		waitGroup.Done()
	}()

	tagger := datadog.NewTagger()
	tagger.Upsert(host, hostTags...)

	// not really useful but doesn't hurt either
	tagger.Print()

	client := datadog.NewClient(host, apiKey, tagger)
	waitGroup.Add(1)
	go func() {
		client.Run(ctx)
		waitGroup.Done()
	}()
	// TODO lifecycle of this chan / create outside ? Wrap ?
	defer close(client.ChanSeries)

	collecterConfig := &collecter.Config{
		MetricsCh:       client.ChanSeries,
		Tagger:          tagger,
		Host:            host,
		CollectInterval: time.Second * 15,
	}

	for i, c := range []collecter.Collecter{
		network.NewARPReporter(collecterConfig.OverrideCollectInterval(time.Second * 10)),
		dnsmasq.NewDnsMasqReporter(collecterConfig),
		load.NewLoadReporter(collecterConfig.OverrideCollectInterval(time.Second * 10)),
		temperature.NewTemperatureReporter(collecterConfig.OverrideCollectInterval(time.Second * 30)),
		network.NewStatisticsReporter(collecterConfig),
		memory.NewMemoryReporter(collecterConfig),
	} {
		select {
		case <-ctx.Done():
			break
		default:
			waitGroup.Add(1)
			go func(i int, coll collecter.Collecter) {
				coll.Collect(ctx)
				waitGroup.Done()
			}(i, c)
			// TODO do we need this poor load spread ?
			time.Sleep(time.Second)
		}
	}
	// TODO: maybe add something else like version
	stableTag := "ts:" + strconv.FormatInt(time.Now().Unix(), 10)
	client.ClientUp(stableTag)
	<-ctx.Done()

	ctxShutdown, _ := context.WithTimeout(context.Background(), time.Second*5)
	_ = client.ClientShutdown(ctxShutdown, stableTag)
	waitGroup.Wait()
	log.Printf("program exit")
}
