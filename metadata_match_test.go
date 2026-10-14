package main

import (
	"math"
	"testing"
)

func TestMetadataPosterMatches(t *testing.T) {
	tests := []struct {
		from, poster string
		want         bool
	}{
		{"Uploader <KBRt@Example.COM>", "kbrt@example.com", true},
		{"kBrtkgWkgN6w@Sm0PzkpP.UhA", "kBrtkgWkgN6w@Sm0PzkpP.UhA", true},
		{"other@example.com", "wanted@example.com", false},
		{"someone@example.com", "", false},
	}
	for _, tt := range tests {
		if got := metadataPosterMatches(tt.from, tt.poster); got != tt.want {
			t.Errorf("poster(%q, %q) = %v, want %v", tt.from, tt.poster, got, tt.want)
		}
	}
}

func TestMetadataSubjectKey(t *testing.T) {
	tests := []struct {
		header, filename, want string
	}{
		{"shared-posting-header", "random.part01", "shared-posting-header"},
		{"ABC.part01", "ABC.part01", "abc"},
		{"ABC.part02", "ABC.part02", "abc"},
		{"video.vol001+02.par2", "video.vol001+02.par2", "video"},
		{"different", "file.part02", "different"},
		{"Bridgerton.S04E01.GERMAN.DL.1080p.WEB.h264-SAUERKRAUT", "Bridgerton.S04E01.GERMAN.DL.1080p.WEB.h264-SAUERKRAUT", "bridgerton.s04.german.dl.1080p.web.h264-sauerkraut"},
		{"Bridgerton.S04E02.GERMAN.DL.1080p.WEB.h264-SAUERKRAUT", "Bridgerton.S04E02.GERMAN.DL.1080p.WEB.h264-SAUERKRAUT", "bridgerton.s04.german.dl.1080p.web.h264-sauerkraut"},
	}
	for _, tt := range tests {
		if got := metadataSubjectKey(tt.header, tt.filename); got != tt.want {
			t.Errorf("subjectKey(%q, %q) = %q, want %q", tt.header, tt.filename, got, tt.want)
		}
	}
}

func TestMetadataSizeDeviation(t *testing.T) {
	if got := metadataSizeDeviation(24810000000, 24810000000); got != 0 {
		t.Fatalf("exact size mismatch: %v", got)
	}
	if got := metadataSizeDeviation(22500000000, 25000000000); math.Abs(got-0.1) > 0.0001 {
		t.Fatalf("10%% deviation = %.5f", got)
	}
	if !math.IsInf(metadataSizeDeviation(0, 100), 1) || !math.IsInf(metadataSizeDeviation(1, 0), 1) {
		t.Fatal("unknown size must not match")
	}
}
