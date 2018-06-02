package ingest

import (
	"fmt"
	"strings"

	"github.com/bikeos/bosd/ingest"
)

func Run(dir string) error {
	ch, err := ingest.NewGPSPackets(dir)
	if err != nil {
		return err
	}

	timemap := make(map[int64][]ingest.GPSPacket)
	macset := make(map[string]struct{})
	for gpkt := range ch {
		t := gpkt.Loc().Fix().Unix()
		if lon := gpkt.Loc().Longitude(); lon != lon {
			panic("expected a location")
		}
		mac := gpkt.Pkt().Src()
		if _, ok := macset[mac]; !ok {
			macset[mac] = struct{}{}
			timemap[t] = append(timemap[t], gpkt)
		}
	}

	fmt.Println("var mapFeatures = [")
	for _, pkts := range timemap {
		macs := []string{}
		for _, pkt := range pkts {
			macs = append(macs, pkt.Pkt().Src())
		}
		s := strings.Join(macs, ",")
		fmt.Printf("new ol.Feature({name : '%s'", s)
		rmc := pkts[0].Loc().Msg()
		fmt.Printf(", geometry: new ol.geom.Point(ol.proj.fromLonLat([%g, %g]))}),\n",
			rmc.Longitude(), rmc.Latitude())
	}
	fmt.Println("]")

	return nil
}
