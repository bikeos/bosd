package daemon

import (
	"sync"
)

type Config struct {
	OutDirPath string
}

type daemon struct {
	cfg   Config
	wg    sync.WaitGroup
	errc  chan error
	donec chan struct{}

	s *store
}

func Run(cfg Config) error {
	errc := make(chan error, 1)
	d := &daemon{
		cfg:   cfg,
		errc:  make(chan error),
		donec: make(chan struct{}),
	}
	go func() { errc <- d.run() }()
	return <-errc
}

func (d *daemon) run() (err error) {
	defer func() {
		close(d.donec)
		d.wg.Wait()
	}()
	if d.s, err = newStore(d.cfg.OutDirPath); err != nil {
		return err
	}
	if err = d.runGPS(); err != nil {
		return err
	}
	if err = d.runWifi(); err != nil {
		return err
	}
	return <-d.errc
}

func (d *daemon) worker(f func() error) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		select {
		case d.errc <- f():
		case <-d.donec:
		}
	}()
}
