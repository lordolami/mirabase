package pbimport

// The remote-S3 import manifest adapter (pocketbase-tenant-import.md):
// "Uploaded S3 record files are not automatically inside a PocketBase
// backup. Require a configured read-only source object-store reference or
// an accompanying object manifest/content export." This adapter implements
// the manifest/content-export path: an `remote-manifest.json` carried by
// the source declares every remote object (key, bucket, size, sha256 and
// the record association), and an accompanying content export directory
// (mirroring the keys) lets the pipeline verify byte coverage. Missing
// remote objects remain VISIBLE issues — an inaccessible bucket is never
// reported as a complete import ("Missing remote objects remain visible
// issues").

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// RemoteManifestEntry is one declared remote object.
type RemoteManifestEntry struct {
	Key          string `json:"key"`
	Bucket       string `json:"bucket"`
	Size         int64  `json:"size"`
	SHA256       string `json:"sha256"`
	CollectionID string `json:"collectionId"`
	RecordID     string `json:"recordId"`
	Filename     string `json:"filename"`
}

// RemoteManifest is the parsed object manifest.
type RemoteManifest struct {
	Entries []RemoteManifestEntry `json:"entries"`
}

// ParseRemoteManifest decodes and validates the manifest: every entry
// requires a key, size >= 0 and a 64-char sha256 ("hashes include
// algorithm/version"; the sha256 field is the hex digest).
func ParseRemoteManifest(raw []byte) (*RemoteManifest, error) {
	var m RemoteManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("%w: the remote manifest is not valid JSON: %v", ErrInvalidRequest, err)
	}
	seen := map[string]bool{}
	for i, e := range m.Entries {
		if strings.TrimSpace(e.Key) == "" {
			return nil, fmt.Errorf("%w: manifest entry %d has no key", ErrInvalidRequest, i)
		}
		if seen[e.Key] {
			return nil, fmt.Errorf("%w: manifest entry %q is duplicated", ErrInvalidRequest, e.Key)
		}
		seen[e.Key] = true
		if e.Size < 0 {
			return nil, fmt.Errorf("%w: manifest entry %q has a negative size", ErrInvalidRequest, e.Key)
		}
		if len(e.SHA256) != 64 || !isHex(e.SHA256) {
			return nil, fmt.Errorf("%w: manifest entry %q has no valid sha256", ErrInvalidRequest, e.Key)
		}
	}
	return &m, nil
}

// findRemoteManifest locates `remote-manifest.json` in the analyzed work
// dir, the source zip's extracted files, or next to the source file.
func (s *Service) findRemoteManifest(id string) (string, bool) {
	candidates := []string{
		filepath.Join(s.SessionDir(id), "work", "remote-manifest.json"),
		filepath.Join(s.SourceDir(id), "remote-manifest.json"),
		filepath.Join(s.SessionDir(id), "remote-manifest.json"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

// RemoteCoverage is the verified remote-object coverage result.
type RemoteCoverage struct {
	Declared      int                   `json:"declaredRemoteFiles"`
	Verified      int                   `json:"verifiedRemoteFiles"`
	Missing       int                   `json:"missingRemoteFiles"`
	RequiredBytes int64                 `json:"remoteRequiredBytes"`
	Entries       []RemoteManifestEntry `json:"entries"`
	MissingKeys   []string              `json:"missingKeys"`
}

// VerifyRemoteCoverage verifies the declared remote objects against the
// accompanying content export (a directory mirroring the object keys).
// With no manifest, coverage is vacuously complete (the source declares
// no remote objects); with a manifest but no export, every declared
// object is missing — "an inaccessible bucket is not reported as a
// complete import".
func (s *Service) VerifyRemoteCoverage(ctx context.Context, id string) (*RemoteCoverage, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return nil, err
	}
	coverage := &RemoteCoverage{MissingKeys: []string{}}

	manifestPath, ok := s.findRemoteManifest(id)
	if !ok {
		// No manifest: the source declares no remote files (local-only
		// source) — vacuously complete.
		return coverage, nil
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	manifest, err := ParseRemoteManifest(raw)
	if err != nil {
		return nil, err
	}
	coverage.Declared = len(manifest.Entries)
	coverage.Entries = manifest.Entries

	// The content export directory (optional): mirrors the object keys.
	exportDir := filepath.Join(s.SessionDir(id), "work", "remote-export")
	exportExists := false
	if info, err := os.Stat(exportDir); err == nil && info.IsDir() {
		exportExists = true
	}

	for _, e := range manifest.Entries {
		coverage.RequiredBytes += e.Size
		if !exportExists {
			// A manifest with NO content export: the objects are declared
			// but not verifiable — missing until the operator supplies the
			// export or the read-only object-store reference resolves.
			coverage.Missing++
			coverage.MissingKeys = append(coverage.MissingKeys, e.Key)
			continue
		}
		objPath := filepath.Join(exportDir, filepath.FromSlash(e.Key))
		f, err := os.Open(objPath)
		if err != nil {
			coverage.Missing++
			coverage.MissingKeys = append(coverage.MissingKeys, e.Key)
			continue
		}
		h := sha256.New()
		var n int64
		if size, err := copyAndCount(h, f); err != nil {
			f.Close()
			coverage.Missing++
			coverage.MissingKeys = append(coverage.MissingKeys, e.Key)
			continue
		} else {
			n = size
		}
		f.Close()
		if n != e.Size {
			coverage.Missing++
			coverage.MissingKeys = append(coverage.MissingKeys, e.Key)
			continue
		}
		got := hex.EncodeToString(h.Sum(nil))
		if got != e.SHA256 {
			coverage.Missing++
			coverage.MissingKeys = append(coverage.MissingKeys, e.Key)
			continue
		}
		coverage.Verified++
	}

	return coverage, nil
}

func copyAndCount(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}
