package main

import (
	"context"
	"fmt"
	"html"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Tensai75/nntp"
	"github.com/Tensai75/nzbparser"
	"github.com/Tensai75/subjectparser"
)

// Keep the opt-in metadata scan bounded even in high-volume groups. Only
// overview headers are read; article bodies are never downloaded.
const metadataMaxPosterHits = metadataMaxCandidates

type metadataCandidate struct {
	key      string
	files    map[string]*nzbparser.NzbFile
	earliest time.Time
	latest   time.Time
}

func (c *metadataCandidate) add(group string, overview nntp.MessageOverview) bool {
	subjectText := html.UnescapeString(strings.ToValidUTF8(overview.Subject, ""))
	parsed, err := subjectparser.Parse(subjectText)
	if err != nil || parsed.Filename == "" || overview.MessageId == "" || overview.Bytes <= 0 {
		return false
	}
	name := strings.ToLower(parsed.Filename)
	file := c.files[name]
	if file == nil {
		file = &nzbparser.NzbFile{
			Groups:       []string{html.EscapeString(group)},
			Poster:       strings.ToValidUTF8(overview.From, ""),
			Subject:      subjectText,
			Number:       parsed.File,
			Filename:     parsed.Filename,
			Basefilename: parsed.Basefilename,
		}
		c.files[name] = file
	}
	if parsed.Segment == 1 {
		file.Subject = subjectText
	}
	date := overview.Date
	if date.IsZero() {
		return false
	}
	if c.earliest.IsZero() || date.Before(c.earliest) {
		c.earliest = date
	}
	if date.After(c.latest) {
		c.latest = date
	}
	if int(date.Unix()) > file.Date {
		file.Date = int(date.Unix())
	}
	// Retries can deliver a chunk twice. Deduplicate by Message-ID when
	// finalizing with nzbparser.MakeUnique().
	file.Segments = append(file.Segments, nzbparser.NzbSegment{
		Number: parsed.Segment,
		Id:     html.EscapeString(strings.Trim(overview.MessageId, "<>")),
		Bytes:  overview.Bytes,
	})
	return true
}

func (c *metadataCandidate) nzb() *nzbparser.Nzb {
	nzb := &nzbparser.Nzb{}
	for _, file := range c.files {
		nzb.Files = append(nzb.Files, *file)
	}
	nzbparser.MakeUnique(nzb)
	nzbparser.ScanNzbFile(nzb)
	sort.Sort(nzb.Files)
	for i := range nzb.Files {
		sort.Sort(nzb.Files[i].Segments)
	}
	return nzb
}

// scanMetadataChunk is retried as a whole on a transport error. A fresh
// connection is selected for each retry, and the group is selected explicitly
// because NNTP connections are stateful.
func scanMetadataChunk(ctx context.Context, group string, first, last uint, output chan<- metadataOverviewHit, scanned *atomic.Uint64) error {
	attempts := conf.Directsearch.OverviewRetries + 1
	if attempts < 1 {
		attempts = 1
	}
	var lastError error
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		conn, err := pool.Get(ctx)
		if err != nil {
			return err
		}
		_, _, _, err = conn.Group(group)
		if err != nil {
			conn.Close()
			pool.Put(conn)
			lastError = err
			continue
		}
		reader, err := conn.OverviewReader(int(first), int(last))
		if err != nil {
			conn.Close()
			pool.Put(conn)
			lastError = err
			continue
		}
		completed := false
		for {
			if err = ctx.Err(); err != nil {
				break
			}
			var line string
			line, err = reader.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimSpace(line)
			if line == "." {
				completed = true
				break
			}
			if line == "" {
				continue
			}
			scanned.Add(1)
			// Parse all headers when explicitly ignoring the poster. During a
			// normal scan, inspect nearby From values too, so a failed
			// poster filter is diagnosable without a second NNTP scan.
			hasPoster := args.Poster != "" && strings.Contains(strings.ToLower(line), strings.ToLower(args.Poster))
			if !args.IgnorePoster && !hasPoster && !args.IsTimestamp {
				continue
			}
			overview, parseErr := nntp.ParseOverviewLine(line)
			if parseErr != nil {
				continue
			}
			posterMatch := metadataPosterMatches(overview.From, args.Poster)
			if !args.IgnorePoster && !posterMatch && !metadataInWindow(overview.Date, args.UnixDate, metadataDiagnosticMinutes*time.Minute) {
				continue
			}
			if overview.Date.IsZero() || overview.Date.Unix() < startDate-3600 || overview.Date.Unix() > endDate+3600 {
				continue
			}
			select {
			case output <- metadataOverviewHit{overview: overview, posterMatch: posterMatch}:
			case <-ctx.Done():
				err = ctx.Err()
			}
			if err != nil {
				break
			}
		}
		if completed {
			pool.Put(conn)
			return nil
		}
		conn.Close()
		pool.Put(conn)
		if err == nil {
			lastError = io.ErrUnexpectedEOF
		} else {
			lastError = err
		}
	}
	return fmt.Errorf("NNTP overview %d-%d failed after %d attempts: %w", first, last, attempts, lastError)
}

// searchMetadata scans by poster, reconstructs multipart files from XOVER
// headers, then requires one unambiguous size+file-count match. Never select
// a plausible-but-unverified candidate merely because it is ranked first.
func searchMetadata(group string, first, last uint) ([]*nzbparser.Nzb, error) {
	if args.Poster == "" || args.ExpectedFiles < 1 || args.ExpectedSizeGB <= 0 {
		return nil, fmt.Errorf("metadata search requires --poster, --expected-files and --expected-size-gb")
	}
	expectedBytes := int64(math.Round(args.ExpectedSizeGB * 1e9))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type chunk struct{ first, last uint }
	jobs := make(chan chunk, 8)
	overviews := make(chan metadataOverviewHit, 4096)
	errorsCh := make(chan error, 1)
	var once sync.Once
	var wg sync.WaitGroup
	var scanned atomic.Uint64

	workers := conf.Directsearch.Connections
	if workers < 1 {
		workers = 1
	}
	if workers > 4 {
		workers = 4
	}
	step := uint(conf.Directsearch.Step)
	if step == 0 {
		step = 20000
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := scanMetadataChunk(ctx, group, job.first, job.last, overviews, &scanned); err != nil {
					once.Do(func() {
						errorsCh <- err
						cancel()
					})
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for cursor := first; cursor <= last; {
			end := cursor + step - 1
			if end < cursor || end > last {
				end = last
			}
			select {
			case <-ctx.Done():
				return
			case jobs <- chunk{cursor, end}:
			}
			if end == last {
				return
			}
			cursor = end + 1
		}
	}()
	go func() {
		wg.Wait()
		close(overviews)
	}()

	candidates := make(map[string]*metadataCandidate)
	nearbyPosterCounts := make(map[string]int)
	matches := 0
	parsedHits := 0
	for hit := range overviews {
		overview := hit.overview
		if metadataInWindow(overview.Date, args.UnixDate, metadataDiagnosticMinutes*time.Minute) {
			nearbyPosterCounts[overview.From]++
		}
		if !hit.posterMatch && !args.IgnorePoster {
			continue
		}
		matches++
		if matches > metadataMaxPosterHits {
			once.Do(func() {
				errorsCh <- fmt.Errorf("metadata scan exceeded %d poster-matching articles; narrow the date interval", metadataMaxPosterHits)
				cancel()
			})
			continue
		}
		subjectText := html.UnescapeString(strings.ToValidUTF8(overview.Subject, ""))
		parsed, err := subjectparser.Parse(subjectText)
		if err != nil {
			continue
		}
		key := metadataCandidateKey(overview.From, metadataSubjectKey(parsed.Header, parsed.Basefilename))
		if key == "" {
			continue
		}
		c := candidates[key]
		if c == nil {
			c = &metadataCandidate{key: key, files: make(map[string]*nzbparser.NzbFile)}
			candidates[key] = c
		}
		if c.add(group, overview) {
			parsedHits++
		}
	}
	select {
	case err := <-errorsCh:
		return nil, err
	default:
	}
	Log.Info("Metadata scan: %d headers scanned, %d candidate article hits, %d parsed, %d candidate groups", scanned.Load(), matches, parsedHits, len(candidates))
	if matches == 0 && !args.IgnorePoster {
		Log.Warn("No matching NNTP From header found for poster %q; this does not prove the post is unavailable", args.Poster)
		if len(nearbyPosterCounts) > 0 {
			Log.Info("Most active NNTP posters within +/- %d minutes of the supplied timestamp (diagnostic, not release matches):", metadataDiagnosticMinutes)
			for _, poster := range metadataTopPosters(nearbyPosterCounts, 10) {
				Log.Info("    %s: %d nearby articles", poster, nearbyPosterCounts[poster])
			}
		} else {
			Log.Warn("No parseable headers near the exact timestamp; verify timestamp timezone and NNTP provider")
		}
		Log.Info("Use --ignore-poster to reconstruct candidates without trusting the indexer's poster field")
	}

	type ranked struct {
		key       string
		nzb       *nzbparser.Nzb
		deviation float64
		fileDelta int
	}
	var rankedCandidates []ranked
	var accepted []ranked
	for _, c := range candidates {
		nzb := c.nzb()
		fileDelta := nzb.Files.Len() - args.ExpectedFiles
		if fileDelta < 0 {
			fileDelta = -fileDelta
		}
		deviation := metadataSizeDeviation(nzb.Bytes, expectedBytes)
		r := ranked{c.key, nzb, deviation, fileDelta}
		rankedCandidates = append(rankedCandidates, r)
		// NZBIndex-like size fields can differ slightly from yEnc overview
		// byte counts. Refuse ambiguous matches rather than guessing.
		timeMatches := true
		if args.IsTimestamp {
			// Indexers may report the start or completion time of a multipart
			// posting. Either boundary must be reasonably close to the supplied
			// precise time, rather than accepting an unrelated upload that day.
			startDistance := math.Abs(float64(c.earliest.Unix() - args.UnixDate))
			endDistance := math.Abs(float64(c.latest.Unix() - args.UnixDate))
			timeMatches = !c.earliest.IsZero() && math.Min(startDistance, endDistance) <= 6*3600
		}
		if fileDelta == 0 && deviation <= 0.15 && timeMatches && nzb.TotalSegments == nzb.Segments {
			accepted = append(accepted, r)
		}
	}
	sort.Slice(rankedCandidates, func(i, j int) bool {
		a, b := rankedCandidates[i], rankedCandidates[j]
		if a.fileDelta != b.fileDelta {
			return a.fileDelta < b.fileDelta
		}
		return a.deviation < b.deviation
	})
	for i, r := range rankedCandidates {
		if i >= 5 {
			break
		}
		Log.Info("Metadata candidate: %q, %d files, %.2f GB, size difference %.1f%%", r.key, r.nzb.Files.Len(), float64(r.nzb.Bytes)/1e9, r.deviation*100)
	}
	if len(accepted) != 1 {
		if len(accepted) > 1 {
			Log.Warn("Metadata scan found %d plausible uploads; refusing to select an ambiguous NZB", len(accepted))
		} else {
			Log.Warn("Metadata scan found no complete, confidently matching upload. Verify poster, timezone, retention and actual article headers.")
		}
		return nil, nil
	}
	Log.Info("Metadata match confirmed: %d files, %.2f GB (%.1f%% size difference)", accepted[0].nzb.Files.Len(), float64(accepted[0].nzb.Bytes)/1e9, accepted[0].deviation*100)
	return []*nzbparser.Nzb{accepted[0].nzb}, nil
}
