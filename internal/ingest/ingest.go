package ingest

import (
	"fmt"
	"strings"

	"github.com/bikeos/bosd/ingest"
)

func Run(dir string) error {
	tdb := ingest.NewTimeMapDB()
	if err := tdb.AddTrips(dir); err != nil {
		return err
	}
	fmt.Println("var mapFeatures = [")
	for _, pkts := range tdb.Packets {
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
