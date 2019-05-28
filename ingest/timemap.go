package ingest

import (
	"encoding/gob"
	"fmt"
	"os"
	"path"
)

type TimeMap map[int64][]GPSPacket

type TimeMapDB struct {
	Trips   map[string]struct{}
	Packets TimeMap
}

func NewTimeMapDB() *TimeMapDB {
	return &TimeMapDB{
		Trips:   make(map[string]struct{}),
		Packets: make(TimeMap),
	}
}

func (tdb *TimeMapDB) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewDecoder(f).Decode(tdb)
}

func (tdb *TimeMapDB) Save(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(*tdb)
}

func (tdb *TimeMapDB) AddTrips(dir string) error {
	if len(tdb.Trips) > 0 {
		// TODO: filter out already processed dirs; load from cache
		panic("STUB")
	}
	trips, err := sortedNames(dir)
	if err != nil {
		return err
	}
	paths := make([]string, len(trips))
	for i := range trips {
		paths[i] = path.Join(dir, trips[i])
	}
	ch, err := NewGPSPackets(paths)
	if err != nil {
		return err
	}
	tm, err := NewTimeMap(ch)
	if err != nil {
		return err
	}
	tdb.Packets = tm
	for _, d := range trips {
		tdb.Trips[d] = struct{}{}
	}
	return nil
}

func NewTimeMap(ch <-chan GPSPacket) (TimeMap, error) {
	timemap, macset := make(TimeMap), make(map[string]struct{})
	for gpkt := range ch {
		t := gpkt.Loc().Fix().Unix()
		if lon := gpkt.Loc().Longitude(); lon != lon {
			return nil, fmt.Errorf("expected a location")
		}
		mac := gpkt.Pkt().Src()
		if _, ok := macset[mac]; !ok {
			macset[mac] = struct{}{}
			timemap[t] = append(timemap[t], gpkt)
		}
	}
	return timemap, nil
}
