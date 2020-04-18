package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/JulienBalestra/metrics/cmd/version"
	"github.com/JulienBalestra/metrics/pkg/collector"
	"github.com/JulienBalestra/metrics/pkg/collector/catalog"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

const (
	datadogAPIKeyFlag         = "datadog-api-key"
	datadogClientSendInterval = "datadog-client-send-interval"
	hostnameFlag              = "hostname"
	defaultCollectionInterval = time.Second * 30
)

func main() {
	root := &cobra.Command{
		Short: "metrics",
		Long:  "metrics",
		Use: `
disable any collector with --collector-${collector} 0s
`,
	}
	root.AddCommand(version.NewCommand())
	fs := &pflag.FlagSet{}

	var hostTagsStrings []string
	hostname, _ := os.Hostname()
	hostname = strings.ToLower(hostname)
	datadogClientConfig := &datadog.Config{
		Host: hostname,
	}

	fs.StringArrayVar(&hostTagsStrings, "datadog-host-tags", nil, "datadog host tags")
	fs.StringVar(&datadogClientConfig.DatadogAPIKey, datadogAPIKeyFlag, os.Getenv("DATADOG_API_KEY"), "datadog API key to submit series")
	fs.StringVar(&hostname, hostnameFlag, hostname, "datadog host tag")
	fs.DurationVar(&datadogClientConfig.SendInterval, datadogClientSendInterval, time.Second*35, "datadog client send interval to the API")

	collectorCatalog := catalog.GetCollectorCatalog()
	collectionDuration := make(map[string]*time.Duration, len(collectorCatalog))
	for name := range collectorCatalog {
		var d time.Duration
		collectionDuration[name] = &d
		fs.DurationVar(&d, "collector-"+name, defaultCollectionInterval, "collection interval for "+name)
	}

	root.Flags().AddFlagSet(fs)
	root.PreRunE = func(cmd *cobra.Command, args []string) error {
		var errorStrings []string
		if datadogClientConfig.DatadogAPIKey == "" {
			errorStrings = append(errorStrings, fmt.Sprintf("flag --%s must be set to a datadog API key", datadogAPIKeyFlag))
		}
		const minimalSendInterval = time.Second * 10
		if datadogClientConfig.SendInterval <= minimalSendInterval {
			errorStrings = append(errorStrings, fmt.Sprintf("flag --%s must be greater or equal to %s", datadogClientSendInterval, minimalSendInterval))
		}
		if hostname == "" {
			errorStrings = append(errorStrings, fmt.Sprintf("empty hostname, flag --%s to define one", hostnameFlag))
		}

		if errorStrings == nil {
			return nil
		}
		return errors.New(strings.Join(errorStrings, "; "))
	}

	root.RunE = func(cmd *cobra.Command, args []string) error {
		hostTags, err := tagger.CreateTags(hostTagsStrings...)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.TODO())
		waitGroup := &sync.WaitGroup{}

		waitGroup.Add(1)
		go func() {
			notifySignals(ctx, cancel)
			waitGroup.Done()
		}()

		tag := tagger.NewTagger()
		tag.Add(hostname, hostTags...)

		// not really useful but doesn't hurt either
		tag.Print()

		datadogClientConfig.Tagger = tag
		client := datadog.NewClient(datadogClientConfig)
		waitGroup.Add(1)
		go func() {
			client.Run(ctx)
			waitGroup.Done()
		}()
		// TODO lifecycle of this chan / create outside ? Wrap ?
		defer close(client.ChanSeries)

		for name, newFn := range collectorCatalog {
			select {
			case <-ctx.Done():
				break
			default:
				d := collectionDuration[name]
				if *d <= 0 {
					log.Printf("ignoring collector %s", name)
					continue
				}
				config := &collector.Config{
					SeriesCh:        client.ChanSeries,
					Tagger:          tag,
					Host:            hostname,
					CollectInterval: *d,
				}
				c := newFn(config)
				waitGroup.Add(1)
				go func(coll collector.Collector) {
					collector.RunCollection(ctx, coll)
					waitGroup.Done()
				}(c)
			}
		}
		tsTag := "ts:" + strconv.FormatInt(time.Now().Unix(), 10)
		revisionTag := "commit:" + version.Revision[:8]
		client.MetricClientUp(tsTag, revisionTag)
		<-ctx.Done()

		ctxShutdown, cancel := context.WithTimeout(context.Background(), time.Second*5)
		_ = client.MetricClientShutdown(ctxShutdown, tsTag, revisionTag)
		cancel()
		waitGroup.Wait()
		log.Printf("program exit")
		return nil
	}
	exitCode := 0
	err := root.Execute()
	if err != nil {
		exitCode = 1
	}
	os.Exit(exitCode)
}
