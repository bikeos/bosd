package ingest

import (
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/bikeos/bosd/wlan"
)

func Interfaces(tripDirs []string) ([]string, error) {
	ifaceset := make(map[string]struct{})
	for _, d := range tripDirs {
		ifaces, err := logInterfaces(d)
		if err != nil {
			return nil, err
		}
		for _, p := range ifaces {
			ifaceset[p] = struct{}{}
		}
	}
	ifaces := make([]string, 0, len(ifaceset))
	for p := range ifaceset {
		ifaces = append(ifaces, p)
	}
	return ifaces, nil
}

func NewIfacePacketChan(tripDirs []string, iface string) (<-chan wlan.Packet, error) {
	pktc := make(chan wlan.Packet, 16)
	go func() {
		defer close(pktc)
		for _, name := range tripDirs {
			d := path.Join(name, "wifi", iface)
			if _, err := os.Stat(d); err != nil {
				continue
			}
			for pkt := range newPacketDirChan(d) {
				pktc <- pkt
			}
		}
	}()
	return pktc, nil
}

func logInterfaces(tripDir string) (ifaces []string, err error) {
	fi, err := ioutil.ReadDir(path.Join(tripDir, "wifi"))
	if err != nil {
		return nil, err
	}
	for _, info := range fi {
		if strings.HasPrefix(info.Name(), "wl") {
			ifaces = append(ifaces, info.Name())
		}
	}
	return ifaces, nil
}

func newPacketDirChan(ifaceDir string) <-chan wlan.Packet {
	names, err := pcapSortedNames(ifaceDir)
	if err != nil {
		ch := make(chan wlan.Packet)
		close(ch)
		return ch
	}
	var paths []string
	for _, name := range names {
		if strings.HasPrefix(name, "pcap") {
			paths = append(paths, path.Join(ifaceDir, name))
		}
	}
	pktc := make(chan wlan.Packet, 32)
	go func() {
		defer close(pktc)
		for _, p := range paths {
			curpktc, perr := wlan.NewPCapFileChan(p)
			if perr != nil {
				return
			}
			for pkt := range curpktc {
				pktc <- pkt
			}
		}
	}()
	return pktc
}
