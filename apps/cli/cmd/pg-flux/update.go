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
)

const (
	githubReleaseAPI   = "https://api.github.com/repos/nex-gen-tech/pg-flux/releases"
	githubDownloadBase = "https://github.com/nex-gen-tech/pg-flux/releases/download"
)

var updateHTTPClient = &http.Client{Timeout: 120 * time.Second}

func cmdUpdate() *cobra.Command {
	var targetVersion string
	c := &cobra.Command{
		Use:   "update",
		Short: "Update pg-flux to the latest (or a specific) release",
		Long: "Download and install a pg-flux release from GitHub, replacing the current binary in-place.\n\n" +
			"Defaults to the latest published release. Use --version to pin to a specific tag.",
		Example: "  pg-flux update\n  pg-flux update --version v0.1.6",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpdate(cmd, targetVersion)
		},
	}
	c.Flags().StringVar(&targetVersion, "version", "", "Target version tag (e.g. v0.1.6); defaults to latest release")
	return c
}

type githubRelease struct {
	TagName string `json:"tag_name"`
}

func fetchRelease(target string) (string, error) {
	var apiURL string
	if target == "" {
		apiURL = githubReleaseAPI + "/latest"
	} else {
		if !strings.HasPrefix(target, "v") {
			target = "v" + target
		}
		apiURL = githubReleaseAPI + "/tags/" + target
	}

	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "pg-flux/"+Version)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := updateHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch release info: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		if target != "" {
			return "", fmt.Errorf("release %q not found on GitHub", target)
		}
		return "", fmt.Errorf("no published releases found on GitHub")
	default:
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", fmt.Errorf("parse release info: %w", err)
	}
	return rel.TagName, nil
}

func downloadAsset(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "pg-flux/"+Version)

	resp, err := updateHTTPClient.Do(req)
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
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "<hash>  <filename>" or "<hash> <filename>"
		parts := strings.Fields(line)
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
		// Match by base name only — handles ./pg-flux, pg-flux, etc.
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
	tmpFile, err := os.CreateTemp(dir, ".pg-flux-update-*")
	if err != nil {
		return fmt.Errorf("create temp file (check write permissions for %s): %w", dir, err)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(binData); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmpFile.Chmod(0o755); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod new binary: %w", err)
	}
	tmpFile.Close()

	if isWindows {
		// Windows: can't delete a running exe, but can rename it.
		// Rename current → .old, then rename tmp → current.
		oldPath := execPath + ".old"
		os.Remove(oldPath) // clean up any leftover from a previous update
		if err := os.Rename(execPath, oldPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("back up current binary: %w", err)
		}
		if err := os.Rename(tmpPath, execPath); err != nil {
			os.Rename(oldPath, execPath) // restore on failure
			os.Remove(tmpPath)
			return fmt.Errorf("install new binary: %w", err)
		}
		os.Remove(oldPath) // best-effort cleanup
	} else {
		if err := os.Rename(tmpPath, execPath); err != nil {
			os.Remove(tmpPath)
			return fmt.Errorf("install new binary (try with sudo if permission denied): %w", err)
		}
	}
	return nil
}

func runUpdate(cmd *cobra.Command, targetVersion string) error {
	out := cmd.OutOrStdout()

	// 1. Resolve target version from GitHub
	fmt.Fprintf(out, "Checking for updates...\n")
	tag, err := fetchRelease(targetVersion)
	if err != nil {
		return err
	}

	// 2. Already on this version?
	if tag == Version {
		fmt.Fprintf(out, "pg-flux %s is already up to date.\n", Version)
		return nil
	}
	if Version == "dev" {
		fmt.Fprintf(out, "Installing pg-flux %s (dev build → release)\n", tag)
	} else {
		fmt.Fprintf(out, "Updating pg-flux %s → %s\n", Version, tag)
	}

	// 3. Determine asset name for the running platform
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

	// 4. Download SHA256SUMS and locate expected checksum
	fmt.Fprintf(out, "Fetching checksums...\n")
	sumsData, err := downloadAsset(baseURL + "/SHA256SUMS")
	if err != nil {
		return fmt.Errorf("download SHA256SUMS: %w", err)
	}
	expectedHash, err := parseSHA256Sums(sumsData, assetName)
	if err != nil {
		return err
	}

	// 5. Download binary archive
	fmt.Fprintf(out, "Downloading %s...\n", assetName)
	archiveData, err := downloadAsset(baseURL + "/" + assetName)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	fmt.Fprintf(out, "Downloaded %.1f MB\n", float64(len(archiveData))/1_000_000)

	// 6. Verify checksum
	sum := sha256.Sum256(archiveData)
	if hex.EncodeToString(sum[:]) != expectedHash {
		return fmt.Errorf("checksum mismatch — download may be corrupted; aborting")
	}
	fmt.Fprintf(out, "Checksum verified\n")

	// 7. Extract binary from archive
	var binData []byte
	if isWindows {
		binData, err = extractFromZip(archiveData, binaryName)
	} else {
		binData, err = extractFromTarGz(archiveData, binaryName)
	}
	if err != nil {
		return fmt.Errorf("extract binary: %w", err)
	}

	// 8. Resolve actual path of the running binary (follow symlinks)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current binary: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	// 9. Atomically replace the binary
	fmt.Fprintf(out, "Installing to %s...\n", execPath)
	if err := installBinary(binData, execPath, isWindows); err != nil {
		return err
	}

	fmt.Fprintf(out, "pg-flux updated to %s\n", tag)
	return nil
}
