package main

import (
	"context"
	"github.com/JulienBalestra/metrics/pkg/collecter"
	"github.com/JulienBalestra/metrics/pkg/collecter/load"
	"github.com/JulienBalestra/metrics/pkg/collecter/network"
	"github.com/JulienBalestra/metrics/pkg/collecter/temperature"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"log"
	"os"
	"os/signal"
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

	client := datadog.NewClient(apiKey)
	waitGroup.Add(1)
	go func() {
		client.Run(ctx)
		waitGroup.Done()
	}()
	// TODO lifecycle of this chan / create outside ? Wrap ?
	defer close(client.ChanSeries)

	tagger := datadog.NewTagger()
	tagger.Upsert(host, hostTags...)

	// not really useful but doesn't hurt either
	tagger.Print()

	collecterConfig := &collecter.Config{
		MetricsCh:       client.ChanSeries,
		Tagger:          tagger,
		Host:            host,
		CollectInterval: time.Second * 15,
	}

	for i, c := range []collecter.Collecter{
		network.NewARPReporter(collecterConfig.OverrideCollectInterval(time.Second * 10)),
		load.NewLoadReporter(collecterConfig),
		temperature.NewTemperatureReporter(collecterConfig),
		network.NewStatisticsReporter(collecterConfig),
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
	<-ctx.Done()
	waitGroup.Wait()
	log.Printf("program exit")
}
