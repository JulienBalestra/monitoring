package forward

import (
	"bytes"
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/JulienBalestra/monitoring/pkg/datadog"
	"go.uber.org/zap"
)

const (
	DatadogZapScheme  = "datadog"
	DatadogZapOutput  = DatadogZapScheme + "://zap"
	bufferSize        = 1000000
	bufferSyncTrigger = bufferSize + (bufferSize * 0.15)
)

type Forwarder struct {
	mu     *sync.Mutex
	buffer *bytes.Buffer

	c *datadog.Client

	lastSync time.Time
}

func (f *Forwarder) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	since := time.Since(f.lastSync)
	nn, err := f.buffer.Write(p)
	bufferLen := f.buffer.Len()
	f.mu.Unlock()
	if err != nil {
		return nn, err
	}
	if since > time.Minute || bufferLen > bufferSyncTrigger {
		return nn, f.Sync()
	}
	return nn, nil
}

func (f *Forwarder) Sync() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	f.mu.Lock()
	defer f.mu.Unlock()
	err := f.c.SendLogs(ctx, f.buffer)
	if err != nil {
		return err
	}
	f.buffer.Reset()
	f.lastSync = time.Now()
	return err
}

func (f *Forwarder) Close() error {
	return f.Sync()
}

func NewDatadogForwarder(c *datadog.Client) func(*url.URL) (zap.Sink, error) {
	return func(_ *url.URL) (zap.Sink, error) {
		return &Forwarder{
			mu:       &sync.Mutex{},
			buffer:   bytes.NewBuffer(make([]byte, 0, bufferSize)),
			c:        c,
			lastSync: time.Now(),
		}, nil
	}
}
