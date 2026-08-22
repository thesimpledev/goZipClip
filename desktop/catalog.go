package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// scannedChannelsPath lists, one per line, the channels whose existing
// VODs have already been cataloged into the download archive.
func scannedChannelsPath() string {
	return filepath.Join(configDir(), "scanned.txt")
}

// vodNamesPath is where yt-dlp appends "id<TAB>upload_date<TAB>title"
// for every VOD it catalogs or downloads, so the archive can be shown
// with readable names.
func vodNamesPath() string {
	return filepath.Join(configDir(), "vods.txt")
}

// IsChannelScanned reports whether the channel's back catalog has been
// recorded in the archive already.
func IsChannelScanned(channel string) bool {
	if channel == "" {
		return false
	}
	// #nosec G304 -- the path is built from the per-user config folder
	data, readErr := os.ReadFile(scannedChannelsPath())
	if readErr != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), channel) {
			return true
		}
	}
	return false
}

// MarkChannelScanned records that the channel's back catalog is in the
// archive.
func MarkChannelScanned(channel string) error {
	if channel == "" {
		return errors.New("channel is empty")
	}
	if IsChannelScanned(channel) {
		return nil
	}
	// #nosec G304 -- the path is built from the per-user config folder
	file, openErr := os.OpenFile(scannedChannelsPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if openErr != nil {
		return openErr
	}
	_, writeErr := file.WriteString(channel + "\n")
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

// ytdlpNamesArgs makes yt-dlp append each VOD's id, date, and title to
// the names file. stage is "" for simulated runs (the catalog) and
// "after_move:" for real downloads, where the default stage would turn
// the download into a simulation.
func ytdlpNamesArgs(stage string) []string {
	return []string{"--print-to-file", stage + "%(id)s\t%(upload_date)s\t%(title)s", vodNamesPath()}
}

// ProcessedVOD is one archive entry with whatever name yt-dlp recorded
// for it.
type ProcessedVOD struct {
	ID    string
	Date  string
	Title string
}

// Label renders the entry for people: date and title when known,
// falling back to the id.
func (v ProcessedVOD) Label() string {
	date := ""
	if len(v.Date) == 8 {
		date = formatUploadDate(v.Date)
	}
	switch {
	case v.Title != "" && date != "":
		return date + "  " + v.Title
	case v.Title != "":
		return v.Title
	case date != "":
		return date + "  (id " + v.ID + ")"
	default:
		return "id " + v.ID
	}
}

// readVODNames loads the names file into a map by id. A missing file
// means no names are known yet.
func readVODNames() map[string]ProcessedVOD {
	names := map[string]ProcessedVOD{}
	// #nosec G304 -- the path is built from the per-user config folder
	data, readErr := os.ReadFile(vodNamesPath())
	if readErr != nil {
		return names
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		id := strings.TrimSpace(parts[0])
		if id == "" {
			continue
		}
		entry := ProcessedVOD{ID: id}
		if len(parts) > 1 {
			entry.Date = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			entry.Title = strings.TrimSpace(parts[2])
		}
		names[id] = entry
	}
	return names
}

// archivedIDs returns the VOD ids in the download archive, in file
// order. A missing archive is empty, not an error.
func archivedIDs() ([]string, error) {
	// #nosec G304 -- the archive path is built from the per-user config folder
	data, readErr := os.ReadFile(archivePath())
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil, nil
		}
		return nil, readErr
	}
	var ids []string
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 {
			ids = append(ids, fields[len(fields)-1])
		}
	}
	return ids, nil
}

// ProcessedVODs lists every archived VOD, newest first; entries with no
// recorded date sort last.
func ProcessedVODs() ([]ProcessedVOD, error) {
	ids, idsErr := archivedIDs()
	if idsErr != nil {
		return nil, idsErr
	}
	names := readVODNames()
	out := make([]ProcessedVOD, 0, len(ids))
	for _, id := range ids {
		entry, known := names[id]
		if !known {
			entry = ProcessedVOD{ID: id}
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return newerVOD(out[i], out[j]) })
	return out, nil
}

// newerVOD orders by date, then by id, newest first. Twitch VOD ids are
// numeric and grow over time, so a longer id is a newer one.
func newerVOD(a, b ProcessedVOD) bool {
	aDate, bDate := dateKey(a.Date), dateKey(b.Date)
	if aDate != bDate {
		return aDate > bDate
	}
	if len(a.ID) != len(b.ID) {
		return len(a.ID) > len(b.ID)
	}
	return a.ID > b.ID
}

// dateKey is the sortable form of a recorded date: YYYYMMDD, or empty
// when yt-dlp had none.
func dateKey(date string) string {
	if len(date) != 8 {
		return ""
	}
	return date
}

// ForgetVOD removes one VOD from the download archive and the names
// file, so the next run downloads and processes it again.
func ForgetVOD(id string) error {
	if id == "" {
		return errors.New("vod id is empty")
	}
	archiveErr := dropLines(archivePath(), func(line string) bool {
		fields := strings.Fields(line)
		return len(fields) > 0 && fields[len(fields)-1] == id
	})
	if archiveErr != nil {
		return archiveErr
	}
	return dropLines(vodNamesPath(), func(line string) bool {
		first, _, _ := strings.Cut(line, "\t")
		return strings.TrimSpace(first) == id
	})
}

// dropLines rewrites path without the lines drop selects, keeping every
// other line byte for byte. A missing file is left alone.
func dropLines(path string, drop func(string) bool) error {
	if path == "" || drop == nil {
		return errors.New("missing path or selector")
	}
	// #nosec G304 -- the path is built from the per-user config folder
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return nil
		}
		return readErr
	}
	lines := strings.SplitAfter(string(data), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if line == "" || !drop(strings.TrimRight(line, "\r\n")) {
			kept = append(kept, line)
		}
	}
	// #nosec G703 -- the only callers pass the archive and names files, both built from the per-user config folder
	return os.WriteFile(path, []byte(strings.Join(kept, "")), 0o600)
}

// ResetArchive deletes the download archive together with the scanned
// channel record and the names file, so the app behaves like a fresh
// install: the next channel save or run catalogs the channel again.
func ResetArchive() error {
	for _, path := range []string{archivePath(), scannedChannelsPath(), vodNamesPath()} {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			return rmErr
		}
	}
	return nil
}

// LookupChannel checks that the configured channel exists on Twitch,
// using nothing but yt-dlp's anonymous metadata. A channel with no
// VODs yet counts as found.
func LookupChannel(ctx context.Context, cfg Config, logf func(string, ...any)) error {
	if ctx == nil || logf == nil {
		return errors.New("missing context or logger")
	}
	if cfg.Channel == "" {
		return errors.New("channel is empty")
	}
	args := []string{
		"--simulate",
		"--flat-playlist",
		"--playlist-items", "1",
		"--print", "%(uploader)s",
		"--no-progress",
		channelVideosURL(cfg),
	}
	cmd := newCommand(ctx, resolveYtdlp(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		logTail(logf, "yt-dlp", out)
		return fmt.Errorf("could not find Twitch channel %q: %s", cfg.Channel, lastLine(out))
	}
	return nil
}

// lastLine returns the last non-empty line of a tool's output, which
// is where yt-dlp puts the reason it stopped.
func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line
		}
	}
	return "no output"
}
