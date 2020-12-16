package signals

import (
	"context"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"

	"go.uber.org/zap"
)

func NotifySignals(ctx context.Context, cancel context.CancelFunc, sigusr1 func()) {
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
				sigusr1()
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
