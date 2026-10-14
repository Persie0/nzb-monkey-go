package main

import (
	"reflect"
	"testing"
	"time"
)

func TestMetadataInWindow(t *testing.T) {
	center := int64(1772172628)
	for _, tt := range []struct {
		offset int64
		want   bool
	}{
		{-901, false},
		{-900, true},
		{0, true},
		{900, true},
		{901, false},
	} {
		got := metadataInWindow(time.Unix(center+tt.offset, 0), center, 15*time.Minute)
		if got != tt.want {
			t.Errorf("offset=%d: got %v want %v", tt.offset, got, tt.want)
		}
	}
	if metadataInWindow(time.Time{}, center, 15*time.Minute) {
		t.Fatal("zero NNTP date is not a valid match")
	}
}

func TestMetadataCandidateKeyIsolatesPosters(t *testing.T) {
	a := metadataCandidateKey("Sender <one@example.org>", "foo.s04")
	b := metadataCandidateKey("Sender <two@example.org>", "foo.s04")
	if a == b {
		t.Fatal("different NNTP From values must not be merged into one NZB")
	}
	if a != metadataCandidateKey("SENDER <ONE@EXAMPLE.ORG>", "foo.s04") {
		t.Fatal("poster matching should be case insensitive")
	}
}

func TestMetadataTopPosters(t *testing.T) {
	got := metadataTopPosters(map[string]int{"third": 1, "second": 5, "first": 5}, 2)
	want := []string{"first", "second"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
