package bench

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bikeos/bosd/wlan"
)

func Run(d time.Duration) error {
	wdevs, werr := wlan.Enumerate()
	if werr != nil {
		return werr
	}
	if len(wdevs) == 0 {
		return fmt.Errorf("no devices")
	}

	wifis := make([]*wlan.Wifi, len(wdevs))
	for i := range wdevs {
		if wifis[i], werr = wlan.NewWifi(wdevs[i]); werr != nil {
			return werr
		}
		if err := wifis[i].Down(); err != nil {
			return err
		}
		if err := wifis[i].Monitor(); err != nil {
			return err
		}
		if err := wifis[i].Up(); err != nil {
			return err
		}
	}

	fs := sharedFreqs(wifis)

	var m sync.RWMutex
	startTime, lastUpdate := time.Now(), time.Now()
	pkts := make([]int, len(wifis))
	var started int32
	var wifiIdx int32
	update := func() {
		m.Lock()
		lastUpdate = time.Now()
		fmt.Printf("\033[%dA", len(wifis)+1)
		for i := 0; i < len(wifis); i++ {
			fmt.Printf("%s: %8d packets\r\n", wifis[i].Name(), pkts[i])
		}
		fmt.Printf("Scanning %2d/%2d channels. time: %5.2fs\r\n",
			atomic.LoadInt32(&wifiIdx),
			len(fs),
			time.Since(startTime).Seconds())
		m.Unlock()
	}
	cb := func(idx int) {
		if atomic.LoadInt32(&started) == 0 {
			return
		}
		pkts[idx]++
		m.RLock()
		needsUpdate := time.Since(lastUpdate) > 100*time.Millisecond
		m.RUnlock()
		if !needsUpdate {
			return
		}
		update()
	}
	fmt.Printf("benchmarking %d devices for %v\n", len(wifis), d)
	for i := 0; i < len(wifis)+1; i++ {
		fmt.Println()
	}

	errc := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		errc <- benchDevs(ctx, wifis, cb)
	}()
	dchan := time.Duration(float64(d) / float64(len(fs)))
	for i := 0; i < len(fs); i++ {
		atomic.AddInt32(&wifiIdx, 1)
		update()
		if err := tuneDevs(wifis, fs[i]); err != nil {
			return err
		}
		atomic.StoreInt32(&started, 1)
		time.Sleep(dchan)
		atomic.StoreInt32(&started, 0)
		update()
	}

	for _, w := range wifis {
		w.Down()
	}

	cancel()
	return <-errc
}

func benchDevs(ctx context.Context, wdevs []*wlan.Wifi, cb func(idx int)) error {
	var wg sync.WaitGroup
	errc := make(chan error, len(wdevs))
	wg.Add(len(wdevs))
	for i := range wdevs {
		go func(n int) {
			defer wg.Done()
			r, err := wdevs[n].TcpdumpReader(ctx)
			if err != nil {
				errc <- err
				return
			}
			rr := bufio.NewReader(r)
			for {
				_, rerr := rr.ReadBytes('\n')
				switch rerr {
				case nil:
					break
				case io.EOF:
					return
				default:
					errc <- rerr
					return
				}
				cb(n)
			}
		}(i)
	}
	wg.Wait()
	select {
	case err := <-errc:
		return err
	default:
		return nil
	}
}

func tuneDevs(ws []*wlan.Wifi, mhz int) error {
	var wg sync.WaitGroup
	wg.Add(len(ws))
	errc := make(chan error, len(ws)+1)
	for i := range ws {
		go func(w *wlan.Wifi) {
			defer wg.Done()
			if err := w.Tune(mhz); err != nil {
				errc <- err
			}
		}(ws[i])
	}
	wg.Wait()
	errc <- nil
	return <-errc
}

func sharedFreqs(ws []*wlan.Wifi) (fs []int) {
	fmap := ws[0].Frequencies()
	for _, w := range ws[1:] {
		wfs := w.Frequencies()
		for f := range fmap {
			if _, ok := wfs[f]; !ok {
				delete(fmap, f)
			}
		}
	}
	for f := range fmap {
		fs = append(fs, f)
	}
	return fs
}
