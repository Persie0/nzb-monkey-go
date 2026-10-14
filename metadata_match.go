package main

import (
	"math"
	"regexp"
	"strings"
)

// Obfuscated multi-file uploads frequently have a distinct filename for each
// archive part but share a common posting header or archive stem.
var archivePartSuffix = regexp.MustCompile(`(?i)(?:\.part[0-9]+|\.vol[0-9]+\+[0-9]+|\.r[0-9]{2,3}|\.rar|\.7z|\.zip|\.par2?|\.nfo|\.sfv)$`)

func metadataPosterMatches(from, poster string) bool {
	poster = strings.ToLower(strings.TrimSpace(poster))
	return poster != "" && strings.Contains(strings.ToLower(from), poster)
}

// A real shared posting header takes precedence over the filename. In the
// subject parser, a missing header is replaced with the file's base filename.
func metadataSubjectKey(header, basefilename string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	basefilename = strings.ToLower(strings.TrimSpace(basefilename))
	if header != "" && header != basefilename {
		return header
	}
	stem := basefilename
	for {
		next := archivePartSuffix.ReplaceAllString(stem, "")
		if next == stem {
			break
		}
		stem = next
	}
	return strings.Trim(stem, ". -_")
}

func metadataSizeDeviation(actual, expected int64) float64 {
	if expected <= 0 || actual <= 0 {
		return math.Inf(1)
	}
	return math.Abs(float64(actual-expected)) / float64(expected)
}
