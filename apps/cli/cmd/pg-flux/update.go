package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	githubReleaseAPI   = "https://api.github.com/repos/nex-gen-tech/pg-flux/releases"
	githubDownloadBase = "https://github.com/nex-gen-tech/pg-flux/releases/download"
)

var updateHTTPClient = &http.Client{Timeout: 120 * time.Second}

// ANSI helpers — only used when stdout is a TTY.
const (
	ansiReset      = "\x1b[0m"
	ansiBold       = "\x1b[1m"
	ansiDim        = "\x1b[2m"
	ansiCyan       = "\x1b[96m"
	ansiGreen      = "\x1b[92m"
	ansiYellow     = "\x1b[93m"
	ansiClearEOL   = "\x1b[K"
	ansiClearBelow = "\x1b[J"
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
)

func cursorUp(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("\x1b[%dA", n)
}

// ── cobra command ────────────────────────────────────────────────────────────

func cmdUpdate() *cobra.Command {
	var targetVersion string
	c := &cobra.Command{
		Use:   "update",
		Short: "Update pg-flux to the latest (or a specific) release",
		Long: "Interactively pick a pg-flux release to install, or pass --version to skip the prompt.\n\n" +
			"Downloads the binary from GitHub, verifies SHA-256, and atomically replaces the running binary.",
		Example: "  pg-flux update                    # interactive version picker\n" +
			"  pg-flux update --version v0.1.6   # install a specific version",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, targetVersion)
		},
	}
	c.Flags().StringVar(&targetVersion, "version", "", "Install a specific version (e.g. v0.1.6); skips the interactive prompt")
	return c
}

// ── GitHub API ───────────────────────────────────────────────────────────────

type ghRelease struct {
	TagName     string `json:"tag_name"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
	PublishedAt string `json:"published_at"`
}

func ghRequest(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pg-flux/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")
	return updateHTTPClient.Do(req)
}

// fetchReleases returns up to limit stable (non-draft, non-prerelease) releases,
// newest first.
func fetchReleases(limit int) ([]ghRelease, error) {
	var out []ghRelease
	for page := 1; len(out) < limit; page++ {
		url := fmt.Sprintf("%s?per_page=100&page=%d", githubReleaseAPI, page)
		resp, err := ghRequest(url)
		if err != nil {
			return nil, fmt.Errorf("fetch releases: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
		}
		var page_rels []ghRelease
		if err := json.NewDecoder(resp.Body).Decode(&page_rels); err != nil {
			resp.Body.Close()
			return nil, fmt.Errorf("parse releases: %w", err)
		}
		resp.Body.Close()

		for _, r := range page_rels {
			if !r.Draft && !r.Prerelease {
				out = append(out, r)
				if len(out) >= limit {
					break
				}
			}
		}
		if len(page_rels) < 100 {
			break
		}
	}
	return out, nil
}

func fetchReleaseByTag(tag string) (string, error) {
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}
	resp, err := ghRequest(githubReleaseAPI + "/tags/" + tag)
	if err != nil {
		return "", fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return "", fmt.Errorf("release %q not found on GitHub", tag)
	default:
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}
	var r ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("parse release: %w", err)
	}
	return r.TagName, nil
}

// ── interactive picker ───────────────────────────────────────────────────────

const pickerWindow = 10 // visible rows at a time

// pickerState holds the mutable display state.
type pickerState struct {
	releases   []ghRelease
	cursor     int // absolute index of highlighted item
	scroll     int // index of top visible item
	current    string
	linesDrawn int
}

func newPickerState(releases []ghRelease, current string) *pickerState {
	s := &pickerState{releases: releases, current: current}
	// Pre-select the current version if it's in the list.
	for i, r := range releases {
		if r.TagName == current {
			s.cursor = i
			break
		}
	}
	// Adjust scroll so cursor is visible.
	if s.cursor >= pickerWindow {
		s.scroll = s.cursor - pickerWindow + 1
	}
	return s
}

func (s *pickerState) render(w io.Writer) {
	var sb strings.Builder

	// Move cursor back to the top of what we last drew.
	if s.linesDrawn > 0 {
		sb.WriteString(cursorUp(s.linesDrawn))
	}

	// ── header ─────────────────────────────────────────────────────────────
	sb.WriteString("\r" + ansiClearEOL)
	sb.WriteString(ansiBold + "  Select a version" + ansiReset +
		ansiDim + "  (↑/↓ move · Enter select · Esc/q cancel)" + ansiReset + "\r\n")

	// ── scroll indicator (top) ─────────────────────────────────────────────
	if s.scroll > 0 {
		sb.WriteString("\r" + ansiClearEOL)
		sb.WriteString(fmt.Sprintf("  %s↑ %d more above%s\r\n", ansiDim, s.scroll, ansiReset))
	} else {
		sb.WriteString("\r" + ansiClearEOL + "\r\n") // blank placeholder to keep height fixed
	}

	// ── items ──────────────────────────────────────────────────────────────
	end := s.scroll + pickerWindow
	if end > len(s.releases) {
		end = len(s.releases)
	}
	for i := s.scroll; i < end; i++ {
		r := s.releases[i]
		sb.WriteString("\r" + ansiClearEOL)

		selected := i == s.cursor
		isLatest := i == 0
		isCurrent := r.TagName == s.current

		if selected {
			sb.WriteString(ansiBold + ansiCyan + "  ▸ " + r.TagName + ansiReset)
		} else {
			sb.WriteString("    " + r.TagName)
		}

		// Right-side annotations
		var tags []string
		if isLatest {
			tags = append(tags, ansiGreen+"latest"+ansiReset+ansiDim)
		}
		if isCurrent {
			tags = append(tags, ansiYellow+"installed"+ansiReset+ansiDim)
		}
		// Trim published_at to date only
		if len(r.PublishedAt) >= 10 {
			tags = append(tags, r.PublishedAt[:10])
		}
		if len(tags) > 0 {
			sb.WriteString("  " + ansiDim + strings.Join(tags, " · ") + ansiReset)
		}
		sb.WriteString("\r\n")
	}

	// Pad to full pickerWindow height so line count stays constant.
	for i := end - s.scroll; i < pickerWindow; i++ {
		sb.WriteString("\r" + ansiClearEOL + "\r\n")
	}

	// ── scroll indicator (bottom) ──────────────────────────────────────────
	remaining := len(s.releases) - end
	if remaining > 0 {
		sb.WriteString("\r" + ansiClearEOL)
		sb.WriteString(fmt.Sprintf("  %s↓ %d more below%s\r\n", ansiDim, remaining, ansiReset))
	} else {
		sb.WriteString("\r" + ansiClearEOL + "\r\n")
	}

	// ── position indicator ─────────────────────────────────────────────────
	sb.WriteString("\r" + ansiClearEOL)
	sb.WriteString(fmt.Sprintf("  %s%d / %d%s\r\n", ansiDim, s.cursor+1, len(s.releases), ansiReset))

	fmt.Fprint(w, sb.String())

	// header + top-scroll + pickerWindow + bottom-scroll + position
	s.linesDrawn = 1 + 1 + pickerWindow + 1 + 1
}

func (s *pickerState) moveUp() {
	if s.cursor > 0 {
		s.cursor--
		if s.cursor < s.scroll {
			s.scroll = s.cursor
		}
	}
}

func (s *pickerState) moveDown() {
	if s.cursor < len(s.releases)-1 {
		s.cursor++
		if s.cursor >= s.scroll+pickerWindow {
			s.scroll = s.cursor - pickerWindow + 1
		}
	}
}

// pickVersion runs the interactive TUI and returns the chosen tag, or "" if
// cancelled. Falls back to the latest tag when stdin is not a terminal.
func pickVersion(releases []ghRelease, current string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return releases[0].TagName, nil // non-interactive: pick latest
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return releases[0].TagName, nil
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	fmt.Fprint(os.Stdout, ansiHideCursor)
	defer fmt.Fprint(os.Stdout, ansiShowCursor+"\r\n")

	state := newPickerState(releases, current)
	state.render(os.Stdout)

	buf := make([]byte, 16)
	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}
		input := buf[:n]

		switch {
		// Enter
		case bytes.Equal(input, []byte{'\r'}), bytes.Equal(input, []byte{'\n'}):
			return state.releases[state.cursor].TagName, nil

		// Esc or 'q' or Ctrl+C
		case bytes.Equal(input, []byte{'\x1b'}),
			bytes.Equal(input, []byte{'q'}),
			bytes.Equal(input, []byte{'\x03'}):
			return "", nil

		// Up arrow  (\x1b[A)
		case len(input) >= 3 && input[0] == '\x1b' && input[1] == '[' && input[2] == 'A',
			bytes.Equal(input, []byte{'k'}): // vim-style
			state.moveUp()

		// Down arrow  (\x1b[B)
		case len(input) >= 3 && input[0] == '\x1b' && input[1] == '[' && input[2] == 'B',
			bytes.Equal(input, []byte{'j'}): // vim-style
			state.moveDown()

		// Page Up (half-window jump)
		case len(input) >= 4 && input[0] == '\x1b' && input[1] == '[' &&
			input[2] == '5' && input[3] == '~':
			for i := 0; i < pickerWindow/2; i++ {
				state.moveUp()
			}

		// Page Down (half-window jump)
		case len(input) >= 4 && input[0] == '\x1b' && input[1] == '[' &&
			input[2] == '6' && input[3] == '~':
			for i := 0; i < pickerWindow/2; i++ {
				state.moveDown()
			}
		}

		state.render(os.Stdout)
	}
}

// ── download / install ───────────────────────────────────────────────────────

func downloadBytes(url string) ([]byte, error) {
	resp, err := ghRequest(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func parseSHA256Sums(data []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) >= 2 && parts[1] == assetName {
			return parts[0], nil
		}
	}
	return "", fmt.Errorf("checksum for %q not found in SHA256SUMS", assetName)
}

func extractFromTarGz(data []byte, binaryName string) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar: %w", err)
		}
		if filepath.Base(hdr.Name) == binaryName {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("%q not found in archive", binaryName)
}

func extractFromZip(data []byte, binaryName string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("zip: %w", err)
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) == binaryName {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("%q not found in zip archive", binaryName)
}

func installBinary(binData []byte, execPath string, isWindows bool) error {
	dir := filepath.Dir(execPath)
	tmp, err := os.CreateTemp(dir, ".pg-flux-update-*")
	if err != nil {
		return fmt.Errorf("create temp file (check write permission for %s): %w", dir, err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(binData); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod binary: %w", err)
	}
	tmp.Close()

	if isWindows {
		oldPath := execPath + ".old"
		os.Remove(oldPath)
		if err := os.Rename(execPath, oldPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("back up current binary: %w", err)
		}
		if err := os.Rename(tmpPath, execPath); err != nil {
			os.Rename(oldPath, execPath)
			os.Remove(tmpPath)
			return fmt.Errorf("install new binary: %w", err)
		}
		os.Remove(oldPath)
	} else {
		if err := os.Rename(tmpPath, execPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("install binary (try sudo if permission denied): %w", err)
		}
	}
	return nil
}

// installVersion downloads and installs the given tag.
func installVersion(out io.Writer, tag string) error {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	isWindows := goos == "windows"

	var assetName, binaryName string
	if isWindows {
		assetName = fmt.Sprintf("pg-flux-windows-%s.zip", goarch)
		binaryName = "pg-flux.exe"
	} else {
		assetName = fmt.Sprintf("pg-flux-%s-%s.tar.gz", goos, goarch)
		binaryName = "pg-flux"
	}

	baseURL := githubDownloadBase + "/" + tag

	fmt.Fprintf(out, "Fetching checksums...\n")
	sumsData, err := downloadBytes(baseURL + "/SHA256SUMS")
	if err != nil {
		return fmt.Errorf("download SHA256SUMS: %w", err)
	}
	expected, err := parseSHA256Sums(sumsData, assetName)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "Downloading %s...\n", assetName)
	archiveData, err := downloadBytes(baseURL + "/" + assetName)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	fmt.Fprintf(out, "Downloaded %.1f MB\n", float64(len(archiveData))/1_000_000)

	sum := sha256.Sum256(archiveData)
	if hex.EncodeToString(sum[:]) != expected {
		return fmt.Errorf("checksum mismatch — download may be corrupted")
	}
	fmt.Fprintf(out, "Checksum verified\n")

	var binData []byte
	if isWindows {
		binData, err = extractFromZip(archiveData, binaryName)
	} else {
		binData, err = extractFromTarGz(archiveData, binaryName)
	}
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	fmt.Fprintf(out, "Installing to %s...\n", execPath)
	if err := installBinary(binData, execPath, isWindows); err != nil {
		return err
	}

	fmt.Fprintf(out, "pg-flux updated to %s\n", tag)
	return nil
}

// ── entry point ──────────────────────────────────────────────────────────────

func runUpdate(cmd *cobra.Command, targetVersion string) error {
	out := cmd.OutOrStdout()

	// Non-interactive path: --version was supplied.
	if targetVersion != "" {
		if !strings.HasPrefix(targetVersion, "v") {
			targetVersion = "v" + targetVersion
		}
		// Validate the tag exists before downloading.
		tag, err := fetchReleaseByTag(targetVersion)
		if err != nil {
			return err
		}
		if tag == Version {
			fmt.Fprintf(out, "pg-flux %s is already installed.\n", Version)
			return nil
		}
		fmt.Fprintf(out, "Installing pg-flux %s...\n", tag)
		return installVersion(out, tag)
	}

	// Interactive path: show picker.
	fmt.Fprintf(out, "Fetching available releases...\n")
	releases, err := fetchReleases(50)
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		return fmt.Errorf("no releases found on GitHub")
	}

	// Erase the "Fetching..." line before drawing the picker.
	if term.IsTerminal(int(os.Stdout.Fd())) {
		fmt.Fprint(os.Stdout, "\r\x1b[K") // \r + clear to EOL
		fmt.Fprint(os.Stdout, cursorUp(1))
	}

	chosen, err := pickVersion(releases, Version)
	if err != nil {
		return err
	}
	if chosen == "" {
		fmt.Fprintf(out, "Update cancelled.\n")
		return nil
	}

	if chosen == Version {
		fmt.Fprintf(out, "pg-flux %s is already installed.\n", Version)
		return nil
	}

	if Version == "dev" {
		fmt.Fprintf(out, "Installing pg-flux %s (dev build → release)\n", chosen)
	} else {
		fmt.Fprintf(out, "Updating pg-flux %s → %s\n", Version, chosen)
	}

	return installVersion(out, chosen)
}
