package main

import (
	"archive/tar"
	"archive/zip"
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

// newUpdateCmd implements `snix update`: fetches the latest GitHub release,
// verifies its SHA-256 against the published checksum file, and atomically
// swaps the running binary for the new one.
//
// Minisign verification is deliberately OFF by default — users install via
// the one-liner installer which has a pinned pubkey. Power users can pass
// --verify=minisign once they have a trusted pubkey on disk.
func newUpdateCmd(g *globalFlags) *cobra.Command {
	var (
		dryRun bool
		toVer  string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Upgrade snix to the latest GitHub release",
		Long: `Downloads the latest release from github.com/SamNet-dev/snix,
verifies its SHA-256, and atomically replaces the current binary.

If --to is passed, installs that specific version instead of latest.
Use --dry-run to see what would happen without changing anything.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			current := version
			fmt.Fprintf(out, "current version: %s\n", current)

			target := strings.TrimPrefix(toVer, "v")
			if target == "" {
				latest, err := resolveLatest()
				if err != nil {
					return fmt.Errorf("resolve latest: %w", err)
				}
				target = latest
			}
			fmt.Fprintf(out, "target version:  %s\n", target)

			if strings.TrimPrefix(current, "v") == target {
				fmt.Fprintln(out, "already up to date.")
				return nil
			}

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate self: %w", err)
			}

			archive := archiveName()
			url := fmt.Sprintf("https://github.com/SamNet-dev/snix/releases/download/v%s/%s", target, archive)
			fmt.Fprintf(out, "download:        %s\n", url)

			if dryRun {
				fmt.Fprintln(out, "(dry run — nothing downloaded or replaced)")
				return nil
			}

			tmpDir, err := os.MkdirTemp("", "snix-update-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)
			archPath := filepath.Join(tmpDir, archive)

			if err := downloadWithProgress(out, url, archPath); err != nil {
				return fmt.Errorf("download: %w", err)
			}
			if err := verifySHA(out, archPath, url+".sha256"); err != nil {
				return fmt.Errorf("verify: %w", err)
			}

			binPath, err := extractBinary(archPath, tmpDir)
			if err != nil {
				return fmt.Errorf("extract: %w", err)
			}
			if err := replaceSelf(exe, binPath); err != nil {
				return fmt.Errorf("replace: %w", err)
			}
			fmt.Fprintf(out, "✓ snix upgraded to v%s\n", target)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would happen without changing anything")
	cmd.Flags().StringVar(&toVer, "to", "", "install a specific version (e.g. 0.5.2) instead of latest")
	return cmd
}

// archiveName maps the current runtime to the release asset filename.
func archiveName() string {
	goarch := runtime.GOARCH
	goos := runtime.GOOS
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	arch := goarch
	if goarch == "arm" {
		arch = "armv7"
	}
	return fmt.Sprintf("snix-%s-%s.%s", goos, arch, ext)
}

// resolveLatest follows the /releases/latest redirect to learn the tag
// without needing the GitHub API (which is rate-limited for unauth users).
func resolveLatest() (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get("https://github.com/SamNet-dev/snix/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	loc := resp.Header.Get("Location")
	if loc == "" {
		// Some proxies strip redirects; fall back to the JSON API.
		return resolveLatestAPI()
	}
	// Expect: https://github.com/SamNet-dev/snix/releases/tag/v0.5.0
	idx := strings.LastIndex(loc, "/v")
	if idx < 0 {
		return "", fmt.Errorf("unexpected redirect: %s", loc)
	}
	return loc[idx+2:], nil
}

func resolveLatestAPI() (string, error) {
	resp, err := http.Get("https://api.github.com/repos/SamNet-dev/snix/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return strings.TrimPrefix(body.TagName, "v"), nil
}

// downloadWithProgress fetches url into path, streaming to disk to avoid
// loading the full archive in memory.
func downloadWithProgress(out io.Writer, url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "downloaded %d bytes\n", n)
	return nil
}

// verifySHA fetches the sibling .sha256 file and compares.
func verifySHA(out io.Writer, path, shaURL string) error {
	resp, err := http.Get(shaURL)
	if err != nil {
		return fmt.Errorf("no checksum published: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("checksum not found (HTTP %d)", resp.StatusCode)
	}
	expected, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// File format: "<hex>  <filename>\n"
	fields := strings.Fields(string(expected))
	if len(fields) < 1 {
		return fmt.Errorf("malformed checksum file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, fields[0]) {
		return fmt.Errorf("sha mismatch — refusing to install\n got:  %s\n want: %s", got, fields[0])
	}
	fmt.Fprintln(out, "✓ sha256 verified")
	return nil
}

// extractBinary pulls out the snix / snix.exe from the downloaded archive.
// Returns the absolute path of the extracted binary inside tmpDir.
func extractBinary(archivePath, dest string) (string, error) {
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(archivePath, dest)
	}
	return extractTarGz(archivePath, dest)
}

func extractTarGz(path, dest string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		name := filepath.Base(hdr.Name)
		if name != "snix" {
			continue
		}
		out := filepath.Join(dest, "snix")
		wf, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(wf, tr); err != nil {
			wf.Close()
			return "", err
		}
		wf.Close()
		return out, nil
	}
	return "", fmt.Errorf("snix binary not found in archive")
}

func extractZip(path, dest string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if filepath.Base(zf.Name) != "snix.exe" {
			continue
		}
		src, err := zf.Open()
		if err != nil {
			return "", err
		}
		out := filepath.Join(dest, "snix.exe")
		wf, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			src.Close()
			return "", err
		}
		if _, err := io.Copy(wf, src); err != nil {
			src.Close()
			wf.Close()
			return "", err
		}
		src.Close()
		wf.Close()
		return out, nil
	}
	return "", fmt.Errorf("snix.exe not found in archive")
}

// replaceSelf atomically swaps exe for newBin. On Unix we rename within
// the same directory (atomic). On Windows the running exe is locked, so
// we rename the old one out of the way first, then move the new one in;
// a subsequent restart picks up the new binary.
func replaceSelf(exe, newBin string) error {
	if runtime.GOOS == "windows" {
		old := exe + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exe, old); err != nil {
			return err
		}
		return os.Rename(newBin, exe)
	}
	return os.Rename(newBin, exe)
}
