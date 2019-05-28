package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bikeos/bosd/ingest"
)

type ServerConfig struct {
	ListenAddr string
	RootDir    string
	LogDir     string
}

type apiHandler struct {
	tdb *ingest.TimeMapDB
}

type namedLatLon struct {
	Name string  `json:"name"`
	Lon  float64 `json:"lon"`
	Lat  float64 `json:"lat"`
}

func (ah *apiHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	resp := make([]namedLatLon, 0, len(ah.tdb.Packets))
	for _, pkts := range ah.tdb.Packets {
		macs := []string{}
		for _, pkt := range pkts {
			macs = append(macs, pkt.Pkt().Src())
		}
		s := strings.Join(macs, ",")
		rmc := pkts[0].Loc().Msg()
		resp = append(
			resp,
			namedLatLon{
				Name: s,
				Lon:  rmc.Longitude(),
				Lat:  rmc.Latitude(),
			})
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Println("http error:", err)
	}
}

func Serve(cfg ServerConfig) error {
	tdb := ingest.NewTimeMapDB()
	if err := tdb.AddTrips(cfg.LogDir); err != nil {
		return err
	}
	mux := http.NewServeMux()
	ah := &apiHandler{tdb: tdb}
	mux.Handle("/api/", ah)
	mux.Handle("/", http.FileServer(http.Dir(cfg.RootDir)))
	fmt.Println("serving http on", cfg.ListenAddr)
	s := &http.Server{Addr: cfg.ListenAddr, Handler: mux}
	return s.ListenAndServe()
}
