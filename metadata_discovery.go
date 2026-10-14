package main

import (
	"sort"
	"strings"
	"time"

	"github.com/Tensai75/nntp"
)

const (
	metadataDefaultWindowHours = 6
	metadataMaxWindowHours     = 24
	metadataDiagnosticMinutes  = 15
	metadataMaxCandidates      = 400000
)

type metadataOverviewHit struct {
	overview   nntp.MessageOverview
	posterMatch bool
}

func metadataCandidateKey(from, subjectKey string) string {
	return strings.ToLower(strings.TrimSpace(from)) + "\x1f" + subjectKey
}

func metadataInWindow(t time.Time, centerUnix int64, window time.Duration) bool {
	if t.IsZero() || centerUnix == 0 {
		return false
	}
	delta := t.Unix() - centerUnix
	seconds := int64(window / time.Second)
	return delta >= -seconds && delta <= seconds
}

// Return the busiest NNTP From values near the supplied posting time.
// This is diagnostic evidence, not proof that any of them belong to a release.
func metadataTopPosters(counts map[string]int, limit int) []string {
	type entry struct {
		poster string
		count  int
	}
	entries := make([]entry, 0, len(counts))
	for poster, count := range counts {
		entries = append(entries, entry{poster, count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].poster < entries[j].poster
	})
	if limit > len(entries) {
		limit = len(entries)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, entries[i].poster)
	}
	return out
}
