package ingest

import (
	"time"

	"github.com/bikeos/bosd/gps"
	"github.com/bikeos/bosd/wlan"
)

type GPSPacket struct {
	loc gps.NMEA
	pkt wlan.Packet
}

func (g *GPSPacket) Loc() gps.NMEA    { return g.loc }
func (g *GPSPacket) Pkt() wlan.Packet { return g.pkt }

func NewGPSPackets(baseDir string) (<-chan GPSPacket, error) {
	ifaces, err := Interfaces(baseDir)
	if err != nil {
		return nil, err
	}

	ifchs := make([]<-chan wlan.Packet, len(ifaces))
	closeifchs := func() {
		for _, ch := range ifchs {
			if ch != nil {
				for range ch {
				}
			}
		}
	}
	for i := range ifchs {
		ch, err := NewIfacePacketChan(baseDir, ifaces[i])
		if err != nil {
			closeifchs()
			return nil, err
		}
		ifchs[i] = ch
	}

	gpsc, err := NewGPSChan(baseDir)
	if err != nil {
		closeifchs()
		return nil, err
	}
	closegpsc := func() {
		for range gpsc {
		}
	}

	ch := make(chan GPSPacket, 64)
	go func() {
		defer closeifchs()
		defer closegpsc()
		defer close(ch)
		bufferPkts(ch, gpsc, ifchs)
	}()

	return ch, nil
}

func bufferPkts(ch chan GPSPacket, gpsc <-chan gps.NMEA, ifchs []<-chan wlan.Packet) {
	pendingPkts := make([]*wlan.Packet, len(ifchs))
	dumpPending := func(n gps.NMEA) (updated bool) {
		now := n.Fix()
		for i := 0; i < len(pendingPkts); i++ {
			if pendingPkts[i] == nil {
				continue
			}
			ptime := pendingPkts[i].Time()
			if ptime.After(now) {
				continue
			}
			if now.Sub(ptime) <= 5*time.Second {
				// Discard if packets too far behind GPS
				ch <- GPSPacket{n, *pendingPkts[i]}
			}
			pendingPkts[i] = nil
			updated = true
		}
		return updated
	}
	for n := range gpsc {
		if n.Fix().IsZero() {
			continue
		}
		if lat := n.Latitude(); lat != lat {
			continue
		}
		for {
			for i := 0; i < len(ifchs); i++ {
				if pendingPkts[i] == nil {
					if p, ok := <-ifchs[i]; ok {
						pendingPkts[i] = &p
					}
				}
			}
			if !dumpPending(n) {
				break
			}
		}
	}
}
