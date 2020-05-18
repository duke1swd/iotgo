/*
 * Format the message into a string
 */

package main

import (
	"sort"
	"strings"
)

// define the order the attributes are placed into the format line.
// -1 means do not place that attribute
// 99 means place it last, even after unknown attributes
const sortLast = 99

var sortOrder = map[string]int{
	"Service":  -1,
	"Location": -1,
	"IOTTime":  0,
	"Seqn":     1,
	"MsgNum":   2,
	"MsgVal":   3,
	"Human":    sortLast,
}

// The sortableKeys type implements sort.Interface
type sortableKeys []string

func (d sortableKeys) Len() int {
	return len(d)
}

func (d sortableKeys) Less(i, j int) bool {
	vi, oki := sortOrder[d[i]]
	vj, okj := sortOrder[d[j]]
	if oki && okj {
		return vi < vj
	} else if oki && !okj {
		return vi != sortLast
	} else if !oki && okj {
		return vi == sortLast
	}
	return strings.Compare(d[i], d[j]) < 0
}

func (d sortableKeys) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func msgFormat(data map[string]string) (s string) {

	// Create a sortableKeys of all the attribute keys in the message,
	// skipping the ones we do not want
	keys := make(sortableKeys, len(data))
	for k := range data {
		v, ok := sortOrder[k]
		if !ok || (ok && v >= 0) {
			keys = append(keys, k)
		}
	}

	// Sort the keys
	sort.Sort(keys)

	// Build the output
	s = ""
	for _, k := range keys {
		if data[k] == "" {
			continue
		}
		if s != "" {
			s += ","
		}
		s += data[k]
	}

	return
}
