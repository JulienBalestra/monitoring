package root

import (
	"context"
	"sync"
	"time"

	"github.com/JulienBalestra/dry/pkg/env"
	"github.com/JulienBalestra/dry/pkg/pidfile"
	"github.com/JulienBalestra/dry/pkg/signals"
	"github.com/JulienBalestra/dry/pkg/version"
	"github.com/JulienBalestra/monitoring/cmd/flags"
	"github.com/JulienBalestra/monitoring/pkg/monitoring"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	defaultPIDFilePath = "/tmp/monitoring.pid"
)

func NewRootCommand(ctx context.Context) *cobra.Command {
	root := &cobra.Command{
		Short: "monitoring application",
		Long:  "monitoring application",
		Use:   "monitoring",
	}
	root.AddCommand(version.NewCommand())
	fs := &pflag.FlagSet{}

	pidFilePath := ""
	pidfile.AddFlag(fs, &pidFilePath, defaultPIDFilePath)

	monitoringConfig := monitoring.NewDefaultConfig()
	flags.AddFlags(fs, monitoringConfig)

	timezone := time.Local.String()
	fs.StringVar(&timezone, "timezone", timezone, "timezone")

	root.Flags().AddFlagSet(fs)
	root.PreRunE = func(cmd *cobra.Command, args []string) error {
		err := env.DefaultFromEnv(&monitoringConfig.DatadogClientConfig.DatadogAPIKey, flags.DatadogAPIKeyFlag, "DATADOG_API_KEY")
		if err != nil {
			return err
		}
		err = env.DefaultFromEnv(&monitoringConfig.DatadogClientConfig.DatadogAPPKey, flags.DatadogAPPKeyFlag, "DATADOG_APP_KEY")
		if err != nil {
			return err
		}
		tz, err := time.LoadLocation(timezone)
		if err != nil {
			return err
		}
		time.Local = tz
		return nil
	}

	root.RunE = func(cmd *cobra.Command, args []string) error {
		err := pidfile.WritePIDFile(pidFilePath)
		if err != nil {
			return err
		}

		m, err := monitoring.NewMonitoring(monitoringConfig)
		if err != nil {
			return err
		}
		runCtx, cancel := context.WithCancel(ctx)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		wg.Add(1)
		go func() {
			signals.NotifySignals(runCtx, m.Tagger.Print)
			cancel()
			wg.Done()
		}()
		return m.Start(runCtx)
	}
	return root
}
