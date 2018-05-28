package daemon

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/bikeos/bosd/audio"
	"github.com/bikeos/bosd/wlan"
)

type report struct {
	s           *store
	devs        map[string]devmacs
	macs        map[string]struct{}
	devsOrdered []string
}

type devmacs map[string]struct{}

func (d *daemon) startReports() error {
	r := report{s: d.s, devs: make(map[string]devmacs), macs: make(map[string]struct{})}
	wdevs, werr := wlan.Enumerate()
	if werr != nil {
		return werr
	}
	for _, wdev := range wdevs {
		r.devs[wdev.Name()] = make(devmacs)
		r.devsOrdered = append(r.devsOrdered, wdev.Name())
	}
	d.worker(func() error { return r.run() })
	return nil
}

func (r *report) run() error {
	for {
		time.Sleep(5 * time.Second)

		olddevmacs := make(map[string]int)
		for dev, macs := range r.devs {
			olddevmacs[dev] = len(macs)
		}
		oldtotal := len(r.macs)

		for dev, macs := range r.devs {
			pcap, err := r.s.WifiCurrentPCap(dev)
			if err != nil {
				continue
			}
			// do better
			cmd := exec.Command("/bin/bash", "-c", "/usr/sbin/tcpdump -n -e -r "+pcap+" | sed 's/[RS]A:/\\nSA /' | grep 'SA '  | cut -f2 -d' ' | sort | uniq")
			tcpOut, err := cmd.Output()
			if len(tcpOut) == 0 {
				continue
			}
			newMacs := strings.Split(string(tcpOut), "\n")
			for _, newMac := range newMacs {
				macs[newMac] = struct{}{}
				r.macs[newMac] = struct{}{}
			}
		}
		say := "radio report: "
		for _, dev := range r.devsOrdered {
			macs := len(r.devs[dev])
			say += fmt.Sprintf("%d macs. gained %d.\n", macs, macs-olddevmacs[dev])
		}
		say += fmt.Sprintf("total unique macs: %d. gained %d.\n", len(r.macs), len(r.macs)-oldtotal)
		audio.Say(context.TODO(), say)
	}
	return nil
}
