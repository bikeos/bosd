package daemon

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/bikeos/bosd/audio"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	OutDirPath string
}

type daemon struct {
	cfg Config
	wg  sync.WaitGroup
	ctx *daemonCtx

	s *store

	gs   gpsStatus
	gsMu sync.RWMutex
}

var reportInterval = 20 * time.Second

func Run(cfg Config) error {
	d := &daemon{
		cfg: cfg,
		ctx: newDaemonCtx(),
	}
	return d.run()
}

func (d *daemon) run() (err error) {
	defer func() {
		d.ctx.Cancel(io.EOF)
		d.wg.Wait()
	}()
	if d.s, err = newStore(d.cfg.OutDirPath); err != nil {
		return err
	}
	if err = d.startGPS(); err != nil {
		return err
	}
	if err = d.startWifi(); err != nil {
		return err
	}
	if err = d.startAudio(); err != nil {
		log.Errorf("audio: %v", err)
		return err
	}

	repc, err := NewReportChannel(d, d.ctx.ctx)
	if err != nil {
		return err
	}

	updatec := make(chan struct{}, 1)
	inputc, err := NewInputChannel(d.ctx.ctx)
	if err != nil {
		d.worker(func(ctx context.Context) error {
			for {
				select {
				case updatec <- struct{}{}:
				case <-ctx.Done():
					return nil
				}
				select {
				case <-time.After(reportInterval):
				case <-ctx.Done():
					return nil
				}
			}
		})
	} else {
		log.Infof("gamepad input detected")
		d.worker(func(ctx context.Context) error {
			for {
				select {
				case _, ok := <-inputc:
					if !ok {
						return io.EOF
					}
				case <-ctx.Done():
					return nil
				}
				select {
				case updatec <- struct{}{}:
				case <-ctx.Done():
					return nil
				}
			}
		})
	}
	d.worker(func(ctx context.Context) error {
		for say := range repc {
			select {
			case <-updatec:
			case <-ctx.Done():
				return nil
			}
			if err := audio.Say(ctx, say); err != nil {
				return err
			}
		}
		return nil
	})
	return <-d.ctx.errc
}

func (d *daemon) worker(f func(ctx context.Context) error) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if err := f(d.ctx.ctx); err != nil {
			audio.Say(d.ctx.ctx, fmt.Sprintf("%v", err))
			d.ctx.Cancel(err)
		}
	}()
}
