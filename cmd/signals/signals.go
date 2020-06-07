package signals

import (
	"context"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"github.com/JulienBalestra/monitoring/pkg/tagger"
	"go.uber.org/zap"
)

func NotifySignals(ctx context.Context, cancel context.CancelFunc, tag *tagger.Tagger) {
	signals := make(chan os.Signal)
	defer close(signals)
	defer signal.Stop(signals)
	defer signal.Reset()

	signal.Notify(signals, syscall.SIGINT, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)
	for {
		select {
		case <-ctx.Done():
			zap.L().Info("end of system signal handling")
			return

		case sig := <-signals:
			zap.L().Info("signal received", zap.String("signal", sig.String()))
			switch sig {
			case syscall.SIGUSR1:
				tag.Print()
			case syscall.SIGUSR2:
				_ = pprof.Lookup("goroutine").WriteTo(os.Stdout, 2)

			case syscall.SIGHUP:
				// nohup

			default:
				cancel()
			}
		}
	}
}
