package daemon

import (
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/bikeos/bosd/wlan"
)

type wifiMon struct {
	devs map[string]*wlan.Wifi
}

var switchTime = 3 * time.Second
var watchDogTime = 30 * time.Second
var bootStaggerTime = 500 * time.Millisecond

func (d *daemon) startWifi() error {
	wm := &wifiMon{devs: make(map[string]*wlan.Wifi)}
	if _, werr := wlan.Enumerate(); werr != nil {
		return werr
	}
	d.worker(func() error { return wm.monDevs(d.s) })
	return nil
}

func (wm *wifiMon) monDevs(s *store) error {
	for {
		wdevs, werr := wlan.Enumerate()
		if werr != nil {
			return werr
		}
		curdevs := make(map[string]wlan.Device)
		for _, wdev := range wdevs {
			curdevs[wdev.Name()] = wdev
		}
		for n := range wm.devs {
			if _, ok := curdevs[n]; !ok {
				/* remove */
				log.Infof("wifi %q removed", n)
			}
		}
		// TODO: check logger if it's bricked
		for n, dev := range curdevs {
			if _, ok := wm.devs[n]; ok {
				continue
			}
			w, err := wlan.NewWifi(dev)
			if err != nil {
				log.Error(err)
			}
			wm.devs[n] = w

			// Booting all devices at once seems to drain
			// a lot of power; play it safe and stagger.
			time.Sleep(bootStaggerTime)

			go wifiLogger(w, s)
		}

		updateTime := switchTime
		if len(wm.devs) > 0 {
			updateTime = watchDogTime
		}
		time.Sleep(updateTime)
	}
	return nil
}

func wifiLogger(w *wlan.Wifi, s *store) (err error) {
	defer func() {
		if cerr := w.Close(); err == nil {
			err = cerr
		}
	}()
	wdir, err := s.Wifi(w.Name())
	if err != nil {
		return err
	}
	if err = w.Down(); err != nil {
		return err
	}
	if err = w.Monitor(); err != nil {
		return err
	}
	if err = w.Up(); err != nil {
		return err
	}

	log.Infof("%s: tcpdump to %s", w.Name(), wdir)
	errc := make(chan error, 1)
	go func() { errc <- w.Tcpdump(wdir) }()

	fs := w.Frequencies()
	freqs := make([]int, 0, len(fs))
	for mhz := range fs {
		freqs = append(freqs, mhz)
	}

	ticker := time.NewTicker(switchTime)
	defer ticker.Stop()
	fidx := 0
	for {
		select {
		case <-ticker.C:
			for i := 0; i < len(freqs); i++ {
				fidx = (fidx + 1) % len(freqs)
				if err = w.Tune(freqs[fidx]); err == nil {
					break
				}
			}
			if err != nil {
				log.Errorf("%d: %v", freqs[fidx], err)
			}
		case <-w.Done():
			err = <-errc
			log.Errorf("%s: %v", w.Name(), err)
			return err
		}
	}
	return nil
}
