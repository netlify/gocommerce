package graceful

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Shutdownable can be closed gracefully
type Shutdownable interface {
	Shutdown(context.Context) error
}

type target struct {
	name    string
	shut    Shutdownable
	timeout time.Duration
}

// Closer handles shutdown of servers and connections
type Closer struct {
	targets      []target
	targetsMutex sync.Mutex

	done     chan struct{}
	doneBool int32
}

// Register inserts a subject to shutdown gracefully
func (cc *Closer) Register(name string, shut Shutdownable, timeout time.Duration) {
	if atomic.LoadInt32(&cc.doneBool) != 0 {
		return
	}

	cc.targetsMutex.Lock()
	cc.targets = append(cc.targets, target{
		name:    name,
		shut:    shut,
		timeout: timeout,
	})
	cc.targetsMutex.Unlock()
}

// DetectShutdown waits for a shutdown signal and then shuts down gracefully
func DetectShutdown(log logrus.FieldLogger) (*Closer, func()) {
	cc := new(Closer)

	go func() {
		WaitForShutdown(log, cc.done)

		if atomic.SwapInt32(&cc.doneBool, 1) != 1 {
			log.Debugf("Initiating shutdown of %d targets", len(cc.targets))
			wg := sync.WaitGroup{}
			cc.targetsMutex.Lock()
			for _, targ := range cc.targets {
				wg.Add(1)
				go func(targ target, log logrus.FieldLogger) {
					defer wg.Done()
					slog := log.WithField("target", targ.name)
					ctx, cancel := context.WithTimeout(context.Background(), targ.timeout)
					defer cancel()
					slog.Debug("Triggering shutdown")
					if err := targ.shut.Shutdown(ctx); err != nil {
						log.WithError(err).Error("Graceful shutdown failed")
					}
					slog.Debug("Shutdown finished")
				}(targ, log.WithField("target", targ.name))
			}
			cc.targetsMutex.Unlock()
			log.Debugln("Waiting for targets to finish shutdown")
			wg.Wait()
			os.Exit(0)
		}
	}()
	return cc, func() {
		cc.done <- struct{}{}
	}
}

// WaitForShutdown blocks until the system signals termination or done has a value
func WaitForShutdown(log logrus.FieldLogger, done <-chan struct{}) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	select {
	case sig := <-signals:
		log.Infof("Triggering shutdown from signal %s", sig)
	case <-done:
		log.Infof("Shutting down...")
	}
}

// ShutdownContext returns a context that is cancelled on termination
func ShutdownContext(ctx context.Context, log logrus.FieldLogger) (context.Context, func()) {
	done := make(chan struct{})
	shut := func() {
		close(done)
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer cancel()
		WaitForShutdown(log, done)
	}()

	return ctx, shut
}
