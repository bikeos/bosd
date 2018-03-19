package wlan

import (
	"os/exec"
	"path/filepath"

	nl80211 "github.com/mdlayher/wifi"
)

type Wifi struct {
	iface   *nl80211.Interface
	tcpdump *exec.Cmd
	donec   chan struct{}
}

func NewWifi(dev Device) (*Wifi, error) {
	return &Wifi{dev.iface, nil, nil}, nil
}

func (w *Wifi) Name() string { return w.iface.Name }

func (w *Wifi) Frequencies() map[int]struct{} {
	return w.iface.Frequencies
}

func (w *Wifi) Down() error { return ifdown(w.iface.Name) }
func (w *Wifi) Up() error   { return ifup(w.iface.Name) }

func (w *Wifi) Close() (err error) {
	if w.tcpdump != nil {
		w.tcpdump.Process.Kill()
		err = w.tcpdump.Wait()
	}
	return err
}

func (w *Wifi) Done() <-chan struct{} {
	return w.donec
}

// Tcpdump writes interface data to a given directory.
func (w *Wifi) Tcpdump(logdir string) error {
	defer func() { close(w.donec) }()
	w.donec = make(chan struct{})
	w.tcpdump = exec.Command(
		"/usr/sbin/tcpdump",
		"-i", w.iface.Name,
		"-w", filepath.Join(logdir, "pcap"),
		"-C", "4",
		"-z", "gzip",
	)
	return w.tcpdump.Run()
}

// Monitor puts the wifi device into monitor mode.
func (w *Wifi) Monitor() error {
	c, err := nl80211.New()
	if err != nil {
		return err
	}
	defer c.Close()
	return c.SetInterface(w.iface, nl80211.InterfaceTypeMonitor)
}

// Tune sets the frequency to some given mhz.
func (w *Wifi) Tune(mhz int) error {
	c, err := nl80211.New()
	if err != nil {
		return err
	}
	defer c.Close()
	return c.SetChannel(w.iface, mhz)
}
