package ingest

import (
	"io/ioutil"
	"sort"
)

func timeSortedNames(d string) ([]string, error) {
	fi, err := ioutil.ReadDir(d)
	if err != nil {
		return nil, err
	}
	sort.Slice(fi[:], func(i, j int) bool {
		return fi[i].ModTime().Before(fi[j].ModTime())
	})
	names := make([]string, len(fi))
	for i, inf := range fi {
		names[i] = inf.Name()
	}
	return names, nil
}
