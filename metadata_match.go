package main

import (
	"math"
	"regexp"
	"strings"
)

// Obfuscated multipart uploads can share a posting header or an archive stem.
var archivePartSuffix = regexp.MustCompile(`(?i)(?:\.part[0-9]+|\.vol[0-9]+\+[0-9]+|\.r[0-9]{2,3}|\.rar|\.7z|\.zip|\.par2?|\.nfo|\.sfv|\.mkv|\.mp4|\.avi|\.m4v|\.ts)$`)
var episodeNumber = regexp.MustCompile(`(?i)(s[0-9]{1,2})e[0-9]{1,3}(?:e[0-9]{1,3})?`)

func metadataPosterMatches(from, poster string) bool {
	poster = strings.ToLower(strings.TrimSpace(poster))
	return poster != "" && strings.Contains(strings.ToLower(from), poster)
}

// The subject parser uses the basename as the header if no explicit header
// exists. Normalize archive numbers and episode numbers in that case.
func metadataSubjectKey(header, basefilename string) string {
	header = strings.ToLower(strings.TrimSpace(header))
	basefilename = strings.ToLower(strings.TrimSpace(basefilename))
	if header != "" && header != basefilename {
		return episodeNumber.ReplaceAllString(header, "1")
	}
	stem := basefilename
	for {
		next := archivePartSuffix.ReplaceAllString(stem, "")
		if next == stem {
			break
		}
		stem = next
	}
	return strings.Trim(episodeNumber.ReplaceAllString(stem, "1"), ". -_")
}

func metadataSizeDeviation(actual, expected int64) float64 {
	if expected <= 0 || actual <= 0 {
		return math.Inf(1)
	}
	return math.Abs(float64(actual-expected)) / float64(expected)
}
