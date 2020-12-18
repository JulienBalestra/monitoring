package flags

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"github.com/JulienBalestra/monitoring/pkg/datadog/forward"
	"github.com/JulienBalestra/monitoring/pkg/monitoring"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

const (
	DatadogAPIKeyFlag         = "datadog-api-key"
	DatadogAPPKeyFlag         = "datadog-app-key"
	DatadogClientSendInterval = "datadog-client-send-interval"

	HostnameFlag = "hostname"
)

func AddFlags(fs *pflag.FlagSet, monitoringConfig *monitoring.Config) {
	hostname, err := os.Hostname()
	if err != nil {
		// TODO
	}
	hostname = strings.ToLower(hostname)

	fs.StringSliceVar(&monitoringConfig.HostTags, "datadog-host-tags", nil, "datadog host tags")
	fs.StringVarP(&monitoringConfig.DatadogClientConfig.DatadogAPIKey, DatadogAPIKeyFlag, "i", "", "datadog API key")
	fs.StringVarP(&monitoringConfig.DatadogClientConfig.DatadogAPPKey, DatadogAPPKeyFlag, "p", "", "datadog APP key")
	fs.StringVar(&monitoringConfig.Hostname, HostnameFlag, hostname, "datadog host tag")
	fs.DurationVar(&monitoringConfig.DatadogClientConfig.SendInterval, DatadogClientSendInterval, time.Second*35, "datadog client send interval to the API >= "+datadog.MinimalSendInterval.String())
	fs.StringVarP(&monitoringConfig.ConfigFile, "config-file", "c", "/etc/monitoring/config.yaml", "monitoring configuration file")
	fs.StringVar(&monitoringConfig.ZapLevel, "log-level", "info", fmt.Sprintf("log level - %s %s %s %s %s %s %s", zap.DebugLevel, zap.InfoLevel, zap.WarnLevel, zap.ErrorLevel, zap.DPanicLevel, zap.PanicLevel, zap.FatalLevel))
	fs.StringSliceVar(&monitoringConfig.ZapConfig.OutputPaths, "log-output", append(monitoringConfig.ZapConfig.OutputPaths, forward.DatadogZapOutput), "log output")
}
