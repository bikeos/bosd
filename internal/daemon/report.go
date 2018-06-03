package daemon

import (
	"context"
	"fmt"
	"math"
	"os/exec"
	"strings"
	"time"

	"github.com/bikeos/bosd/audio"
	"github.com/bikeos/bosd/wlan"
)

type report struct {
	d *daemon

	devs        map[string]devmacs
	macs        map[string]struct{}
	devsOrdered []string
	dist        float64 // sum of haversines
	firstGPS    gpsStatus
	lastGPS     gpsStatus
}

type devmacs map[string]struct{}

var reportInterval = 10 * time.Second

func (d *daemon) startReports() error {
	r := report{d: d, devs: make(map[string]devmacs), macs: make(map[string]struct{})}
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
		time.Sleep(reportInterval)

		olddevmacs := make(map[string]int)
		for dev, macs := range r.devs {
			olddevmacs[dev] = len(macs)
		}
		oldtotal := len(r.macs)

		for dev, macs := range r.devs {
			pcap, err := r.d.s.WifiCurrentPCap(dev)
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

		stuck := 0
		for _, dev := range r.devsOrdered {
			macs := len(r.devs[dev])
			if newMacs := macs - olddevmacs[dev]; newMacs == 0 {
				stuck++
			}
		}

		r.sayReport(len(r.macs)-oldtotal, stuck)
	}
	return nil
}

func deg2rad(deg float64) float64 { return 2 * math.Pi * deg / 360.0 }

func haversine(last, cur gpsStatus) float64 {
	latDelta := cur.lat - last.lat
	lonDelta := cur.lon - last.lon
	phi1, phi2 := deg2rad(last.lat), deg2rad(cur.lat)
	delPhi, delLambda := deg2rad(latDelta), deg2rad(lonDelta)
	a := math.Sin(delPhi/2.0)*math.Sin(delPhi/2.0) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(delLambda/2.0)*math.Sin(delLambda/2.0)
	return 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
}

func delta2bearing(lonDelta, latDelta float64) string {
	say := ""
	atanAbs := math.Abs(math.Atan(lonDelta / latDelta))
	if atanAbs > math.Pi/8.0 {
		// angle somewhat above y-axis
		if lonDelta > 0 {
			say += "north "
		} else {
			say += "south "
		}
	}
	if atanAbs < math.Pi/2.0-math.Pi/8.0 {
		// angle somewhat close to x-axis
		if latDelta > 0 {
			say += "west "
		} else {
			say += "east "
		}
	}
	return say
}

func (r *report) sayReport(gained int, stuck int) {
	r.d.gsMu.RLock()
	curGPS := r.d.gs
	r.d.gsMu.RUnlock()

	if r.firstGPS.when.IsZero() {
		r.firstGPS = curGPS
	}

	say := "radio report: "
	say += fmt.Sprintf("total unique macs: %d. gained %d.\n", len(r.macs), gained)
	if stuck != 0 {
		say += fmt.Sprintf("devices stuck!\n")
	}
	if curGPS.when.IsZero() || curGPS.when.Sub(r.lastGPS.when) < reportInterval {
		say += "GPS stuck."
	} else if r.lastGPS.lat == r.lastGPS.lat {
		Rmi := 3959.0
		hs := haversine(r.lastGPS, curGPS)
		r.dist += hs
		say += fmt.Sprintf(". traveled: %.3g miles.", Rmi*r.dist)

		hs = haversine(r.firstGPS, curGPS)
		say += fmt.Sprintf(". home: %.3g miles.", Rmi*hs)

		// This wasn't helpful on rides:

		// lonDelta := curGPS.lon - r.lastGPS.lon
		// latDelta := curGPS.lat - r.lastGPS.lat
		// say += "Bearing: " + delta2bearing(lonDelta, latDelta)

		// dmi := Rmi * hs
		// dt := curGPS.when.Sub(r.lastGPS.when)
		// say += fmt.Sprintf(
		//	". traveled %d miles per hour.",
		//	int(dmi*(time.Hour.Seconds()/dt.Seconds())))
	}

	audio.Say(context.TODO(), say)

	r.lastGPS = curGPS
}
