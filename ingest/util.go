package ingest

import (
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
)

func sortedNames(d string) ([]string, error) {
	fi, err := ioutil.ReadDir(d)
	if err != nil {
		return nil, err
	}
	sort.Slice(fi[:], func(i, j int) bool {
		return strings.Compare(fi[i].Name(), fi[j].Name()) < 0
	})
	names := make([]string, len(fi))
	for i, inf := range fi {
		names[i] = inf.Name()
	}
	return names, nil
}

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

func pcapSortedNames(d string) ([]string, error) {
	fi, err := ioutil.ReadDir(d)
	if err != nil {
		return nil, err
	}
	sort.Slice(fi[:], func(i, j int) bool {
		iN := strings.Split(fi[i].Name()[4:], ".")[0]
		jN := strings.Split(fi[j].Name()[4:], ".")[0]
		if len(iN) == 0 {
			return true
		}
		if len(jN) == 0 {
			return false
		}
		iV, _ := strconv.ParseInt(iN, 10, 32)
		jV, _ := strconv.ParseInt(jN, 10, 32)
		return iV < jV
	})
	names := make([]string, len(fi))
	for i, inf := range fi {
		names[i] = inf.Name()
	}
	return names, nil
}
