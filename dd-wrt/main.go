package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
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
	datadogCollector "github.com/JulienBalestra/metrics/pkg/collector/datadog"
	"github.com/JulienBalestra/metrics/pkg/datadog"
	"github.com/JulienBalestra/metrics/pkg/tagger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func notifySystemSignals(ctx context.Context, cancel context.CancelFunc) {
	signals := make(chan os.Signal)
	defer close(signals)
	defer signal.Stop(signals)
	defer signal.Reset()

	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of system signal handling")
			return

		case sig := <-signals:
			log.Printf("signal %s received", sig)
			cancel()
		}
	}
}

func notifyUSRSignals(ctx context.Context, tag *tagger.Tagger) {
	signals := make(chan os.Signal)
	defer close(signals)
	defer signal.Stop(signals)
	defer signal.Reset()

	signal.Notify(signals, syscall.SIGUSR1)
	for {
		select {
		case <-ctx.Done():
			log.Printf("end of USR signal handling")
			return

		case sig := <-signals:
			log.Printf("signal %s received", sig)
			tag.Print()
		}
	}
}

const (
	datadogAPIKeyFlag         = "datadog-api-key"
	datadogClientSendInterval = "datadog-client-send-interval"
	hostnameFlag              = "hostname"
	defaultCollectionInterval = time.Second * 30
	minimalSendInterval       = time.Second * 10
	defaultPIDFilePath        = "/tmp/metrics.pid"
)

func main() {
	root := &cobra.Command{
		Short: "metrics for dd-wrt routers",
		Long:  "metrics for dd-wrt routers, disable any collector with --collector-${collector}=0s",
		Use:   "metrics",
	}
	root.AddCommand(version.NewCommand())
	fs := &pflag.FlagSet{}

	var hostTagsStrings []string
	pidFilePath := ""
	hostname, _ := os.Hostname()
	timezone := time.Local.String()
	hostname = strings.ToLower(hostname)
	datadogClientConfig := &datadog.Config{
		Host:          hostname,
		ClientMetrics: &datadog.ClientMetrics{},
	}

	fs.StringSliceVar(&hostTagsStrings, "datadog-host-tags", nil, "datadog host tags")
	fs.StringVar(&pidFilePath, "pid-file", defaultPIDFilePath, "file to write process id")
	fs.StringVarP(&datadogClientConfig.DatadogAPIKey, datadogAPIKeyFlag, "i", "", "datadog API key to submit series")
	fs.StringVar(&hostname, hostnameFlag, hostname, "datadog host tag")
	fs.StringVar(&timezone, "timezone", timezone, "timezone")
	fs.DurationVar(&datadogClientConfig.SendInterval, datadogClientSendInterval, time.Second*35, "datadog client send interval to the API >= "+minimalSendInterval.String())

	collectorCatalog := catalog.CollectorCatalog()
	collectorCatalog[datadogCollector.CollectorName] = func(config *collector.Config) collector.Collector {
		d := datadogCollector.NewClient(config)
		d.ClientMetrics = datadogClientConfig.ClientMetrics
		return d
	}
	collectionDuration := make(map[string]*time.Duration, len(collectorCatalog))
	for name := range collectorCatalog {
		var d time.Duration
		collectionDuration[name] = &d
		fs.DurationVar(&d, "collector-"+name, defaultCollectionInterval, "collection interval/backoff for "+name)
	}

	root.Flags().AddFlagSet(fs)
	root.PreRunE = func(cmd *cobra.Command, args []string) error {
		var errorStrings []string
		if datadogClientConfig.DatadogAPIKey == "" {
			datadogClientConfig.DatadogAPIKey = os.Getenv("DATADOG_API_KEY")
			if datadogClientConfig.DatadogAPIKey == "" {
				errorStrings = append(errorStrings, fmt.Sprintf("flag --%s or envvar DATADOG_API_KEY must be set to a datadog API key", datadogAPIKeyFlag))
			} else {
				log.Printf("using environment variable DATADOG_API_KEY")
			}
		}
		if datadogClientConfig.SendInterval < minimalSendInterval {
			errorStrings = append(errorStrings, fmt.Sprintf("flag --%s must be greater or equal to %s", datadogClientSendInterval, minimalSendInterval))
		}
		if hostname == "" {
			errorStrings = append(errorStrings, fmt.Sprintf("empty hostname, flag --%s to define one", hostnameFlag))
		}
		tz, err := time.LoadLocation(timezone)
		if err != nil {
			errorStrings = append(errorStrings, err.Error())
		}
		if errorStrings == nil {
			time.Local = tz
			return nil
		}
		return errors.New(strings.Join(errorStrings, "; "))
	}

	root.RunE = func(cmd *cobra.Command, args []string) error {
		hostTags, err := tagger.CreateTags(hostTagsStrings...)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(pidFilePath, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			return err
		}
		log.Printf("pid %d to %s", os.Getpid(), pidFilePath)

		ctx, cancel := context.WithCancel(context.TODO())
		waitGroup := &sync.WaitGroup{}

		waitGroup.Add(1)
		go func() {
			notifySystemSignals(ctx, cancel)
			waitGroup.Done()
		}()

		tag := tagger.NewTagger()
		tag.Add(hostname, hostTags...)

		waitGroup.Add(1)
		go func() {
			notifyUSRSignals(ctx, tag)
			waitGroup.Done()
		}()

		// not really useful but doesn't hurt either
		tag.Print()

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
		client.MetricClientUp(hostname, tsTag, revisionTag)
		<-ctx.Done()

		ctxShutdown, cancel := context.WithTimeout(context.Background(), time.Second*5)
		_ = client.MetricClientShutdown(ctxShutdown, hostname, tsTag, revisionTag)
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
