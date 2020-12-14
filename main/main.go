package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/cmd/env"
	"github.com/JulienBalestra/monitoring/cmd/signals"
	"github.com/JulienBalestra/monitoring/cmd/version"
	"github.com/JulienBalestra/monitoring/pkg/collector"
	"github.com/JulienBalestra/monitoring/pkg/collector/catalog"
	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"github.com/JulienBalestra/monitoring/pkg/datadog/forward"
	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	datadogAPIKeyFlag         = "datadog-api-key"
	datadogAPPKeyFlag         = "datadog-app-key"
	datadogClientSendInterval = "datadog-client-send-interval"
	hostnameFlag              = "hostname"
	defaultPIDFilePath        = "/tmp/monitoring.pid"
)

func main() {
	zapConfig := zap.NewProductionConfig()
	zapConfig.OutputPaths = append(zapConfig.OutputPaths, forward.DatadogZapOutput)
	zapLevel := zapConfig.Level.String()

	root := &cobra.Command{
		Short: "monitoring application",
		Long:  "monitoring application",
		Use:   "monitoring",
	}
	root.AddCommand(version.NewCommand())
	fs := &pflag.FlagSet{}

	var hostTagsStrings []string
	pidFilePath := ""
	hostname, _ := os.Hostname()
	timezone := time.Local.String()
	hostname = strings.ToLower(hostname)
	configFile := ""
	var client *datadog.Client
	datadogClientConfig := &datadog.Config{
		Host:          hostname,
		ClientMetrics: &datadog.ClientMetrics{},
	}

	fs.StringSliceVar(&hostTagsStrings, "datadog-host-tags", nil, "datadog host tags")
	fs.StringVar(&pidFilePath, "pid-file", defaultPIDFilePath, "file to write process id")
	fs.StringVarP(&datadogClientConfig.DatadogAPIKey, datadogAPIKeyFlag, "i", "", "datadog API key")
	fs.StringVarP(&datadogClientConfig.DatadogAPPKey, datadogAPPKeyFlag, "p", "", "datadog APP key")
	fs.StringVar(&hostname, hostnameFlag, hostname, "datadog host tag")
	fs.StringVar(&timezone, "timezone", timezone, "timezone")
	fs.DurationVar(&datadogClientConfig.SendInterval, datadogClientSendInterval, time.Second*35, "datadog client send interval to the API >= "+datadog.MinimalSendInterval.String())
	fs.StringVar(&zapLevel, "log-level", zapLevel, fmt.Sprintf("log level - %s %s %s %s %s %s %s", zap.DebugLevel, zap.InfoLevel, zap.WarnLevel, zap.ErrorLevel, zap.DPanicLevel, zap.PanicLevel, zap.FatalLevel))
	fs.StringVarP(&configFile, "config-file", "c", "/etc/monitoring/config.yaml", "monitoring configuration file")
	fs.StringSliceVar(&zapConfig.OutputPaths, "log-output", zapConfig.OutputPaths, "log output")

	root.Flags().AddFlagSet(fs)
	root.PreRunE = func(cmd *cobra.Command, args []string) error {
		err := env.DefaultFromEnv(&datadogClientConfig.DatadogAPIKey, datadogAPIKeyFlag, "DATADOG_API_KEY")
		if err != nil {
			return err
		}
		err = env.DefaultFromEnv(&datadogClientConfig.DatadogAPPKey, datadogAPPKeyFlag, "DATADOG_APP_KEY")
		if err != nil {
			return err
		}
		if datadogClientConfig.SendInterval <= datadog.MinimalSendInterval {
			return fmt.Errorf("flag --%s must be greater or equal to %s", datadogClientSendInterval, datadog.MinimalSendInterval)
		}
		if hostname == "" {
			return fmt.Errorf("empty hostname, flag --%s to define one", hostnameFlag)
		}
		err = zapConfig.Level.UnmarshalText([]byte(zapLevel))
		if err != nil {
			return err
		}
		client = datadog.NewClient(datadogClientConfig)
		// TODO make it works
		err = zap.RegisterSink(forward.DatadogZapScheme, forward.NewDatadogForwarder(client))
		if err != nil {
			return err
		}
		logger, err := zapConfig.Build()
		if err != nil {
			return err
		}
		logger = logger.With(zap.Int("pid", os.Getpid()))
		zap.ReplaceGlobals(logger)
		zap.RedirectStdLog(logger)

		tz, err := time.LoadLocation(timezone)
		if err != nil {
			return err
		}
		time.Local = tz
		return nil
	}

	root.RunE = func(cmd *cobra.Command, args []string) error {
		// validate host tags
		_, err := tagger.CreateTags(hostTagsStrings...)
		if err != nil {
			return err
		}

		catalogConfig, err := catalog.ParseConfigFile(configFile)
		if err != nil {
			return err
		}

		err = ioutil.WriteFile(pidFilePath, []byte(strconv.Itoa(os.Getpid())), 0644)
		if err != nil {
			return err
		}
		zap.L().Info("wrote pid file", zap.Int("pid", os.Getpid()), zap.String("file", pidFilePath))

		ctx, cancel := context.WithCancel(context.TODO())
		waitGroup := &sync.WaitGroup{}

		tag := tagger.NewTagger()
		waitGroup.Add(1)
		go func() {
			signals.NotifySignals(ctx, cancel, tag)
			waitGroup.Done()
		}()

		waitGroup.Add(1)
		go func() {
			client.Run(ctx)
			waitGroup.Done()
		}()
		// TODO lifecycle of this chan / create outside ? Wrap ?
		defer close(client.ChanSeries)

		errorsChan := make(chan error, len(catalogConfig.Collectors))
		defer close(errorsChan)

		for name, newFn := range catalog.CollectorCatalog() {
			select {
			case <-ctx.Done():
				break
			default:
				cf, ok := catalogConfig.Collectors[name]
				if !ok {
					zap.L().Info("ignoring collector", zap.String("collector", name))
					continue
				}
				if cf.Interval <= 0 {
					zap.L().Warn("ignoring collector", zap.String("collector", name))
					continue
				}
				config := &collector.Config{
					MetricsClient:   client,
					Tagger:          tag,
					Host:            hostname,
					CollectInterval: cf.Interval,
					Options:         cf.Options,
				}
				c := newFn(config)
				waitGroup.Add(1)
				go func(coll collector.Collector) {
					errorsChan <- collector.RunCollection(ctx, coll)
					waitGroup.Done()
				}(c)
			}
		}
		tags := append(tag.GetUnstable(hostname),
			"ts:"+strconv.FormatInt(time.Now().Unix(), 10),
			"commit:"+version.Commit[:8],
		)
		client.MetricClientUp(hostname, tags...)
		_ = client.UpdateHostTags(ctx, hostTagsStrings)
		select {
		case <-ctx.Done():
		case err := <-errorsChan:
			zap.L().Error("failed to run collection", zap.Error(err))
			cancel()
		}

		ctxShutdown, cancel := context.WithTimeout(context.Background(), time.Second*5)
		_ = client.MetricClientShutdown(ctxShutdown, hostname, tags...)
		cancel()
		waitGroup.Wait()
		return nil
	}
	exitCode := 0
	err := root.Execute()
	if err != nil {
		exitCode = 1
		zap.L().Error("program exit", zap.Error(err), zap.Int("exitCode", exitCode))
	} else {
		zap.L().Info("program exit", zap.Int("exitCode", exitCode))
	}
	_ = zap.L().Sync()
	os.Exit(exitCode)
}
