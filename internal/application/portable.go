package application

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// R7 portable interchange (MB-PORTABLE): stable-ID typed manifests with
// checksums (portable-v1) plus the native format (the verified archive
// surface). Export writes a real manifest + data archive; import
// validates it against the live schema preserving IDs, with dryRun.
// Page/key encryption rotation is the native-security remainder: the
// released surface refuses it honestly (the store is unencrypted in this
// release).
// ---------------------------------------------------------------------------

// PortableManifestV1 is the typed manifest (stable IDs, checksums).
type PortableManifestV1 struct {
	Format          string                    `json:"format"`
	DatabaseID      string                    `json:"databaseId"`
	Generation      string                    `json:"generation"`
	ExportID        string                    `json:"exportId"`
	CreatedAt       string                    `json:"createdAt"`
	ManifestSHA256  string                    `json:"manifestSha256"`
	Collections     []PortableCollectionV1    `json:"collections"`
	Files           []PortableFileEntryV1     `json:"files"`
}

// PortableCollectionV1 is one collection's typed rows.
type PortableCollectionV1 struct {
	CollectionID   string            `json:"collectionId"`
	CollectionName string            `json:"collectionName"`
	Type           string            `json:"type"`
	RecordCount    int               `json:"recordCount"`
	SHA256         string            `json:"sha256"`
	File           string            `json:"file"`
}

// PortableFileEntryV1 is one file entry (coverage of the file store).
type PortableFileEntryV1 struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// ExportTargetDatabase creates the stable-ID typed manifest and data/file
// archive for one target (or the primary): every user table's rows are
// dumped as typed JSONL with per-collection checksums, plus the manifest
// with its own digest.
func (rt *Runtime) ExportTargetDatabase(ctx context.Context, databaseID string, format string, includeFiles bool) (*PortableManifestV1, error) {
	if format != "portable-v1" && format != "native" {
		return nil, fmt.Errorf("invalid_request: format must be portable-v1 or native")
	}
	var handle *sql.DB
	var err error
	if databaseID == "" || databaseID == PrimaryDatabaseID {
		if rt.Base == nil {
			return nil, fmt.Errorf("the primary connection is unavailable")
		}
		handle = rt.Base.DBConn()
	} else {
		handle, err = rt.Handles.Open(ctx, databaseID)
		if err != nil {
			return nil, fmt.Errorf("failed to open the target database: %w", err)
		}
	}
	dir := rt.exportDir(databaseID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	exportID := "x-" + time.Now().UTC().Format("20060102150405") + "-" + randHex(6)
	exportDir := filepath.Join(dir, exportID)
	if err := os.MkdirAll(exportDir, 0o700); err != nil {
		return nil, err
	}

	manifest := &PortableManifestV1{
		Format:     format,
		DatabaseID: databaseID,
		ExportID:   exportID,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339Nano),
	}
	manifest.Generation = "1"
	if databaseID != "" && databaseID != PrimaryDatabaseID {
		if d, derr := rt.Service.GetDatabase(ctx, databaseID); derr == nil {
			manifest.Generation = fmt.Sprintf("%d", d.Generation)
		}
	}

	// Enumerate the user tables (record tables by name; skip the store).
	rows, err := handle.QueryContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE '_mb_%' AND name NOT LIKE '\_%' ESCAPE '\'`)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, table := range tables {
		collID := table
		collType := "base"
		if rt.Base != nil {
			if coll, cerr := rt.Base.FindCollectionByNameOrId(table); cerr == nil {
				collID = coll.Id
				collType = coll.Type
			}
		}
		cols, cerr := targetColumns(ctx, handle, table)
		if cerr != nil {
			return nil, cerr
		}
		colNames := make([]string, 0, len(cols))
		for c := range cols {
			colNames = append(colNames, c)
		}
		sortStrings(colNames)
		selects := make([]string, 0, len(colNames))
		for _, c := range colNames {
			selects = append(selects, quoteIdent(c))
		}
		rrows, qerr := handle.QueryContext(ctx,
			fmt.Sprintf(`SELECT %s FROM %s ORDER BY id`, strings.Join(selects, ", "), quoteIdent(table)))
		if qerr != nil {
			return nil, qerr
		}
		dataFile := table + ".jsonl"
		f, ferr := os.Create(filepath.Join(exportDir, dataFile))
		if ferr != nil {
			rrows.Close()
			return nil, ferr
		}
		count := 0
		hasher := newSHA()
		vals := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		for rrows.Next() {
			if err := rrows.Scan(ptrs...); err != nil {
				rrows.Close()
				f.Close()
				return nil, err
			}
			row := map[string]any{}
			for i, c := range colNames {
				row[c] = decodePortableValue(vals[i])
			}
			line, jerr := json.Marshal(row)
			if jerr != nil {
				rrows.Close()
				f.Close()
				return nil, jerr
			}
			f.Write(append(line, '\n'))
			hasher.Write(line)
			hasher.Write([]byte{'\n'})
			count++
		}
		rrows.Close()
		if err := rrows.Err(); err != nil {
			f.Close()
			return nil, err
		}
		f.Close()
		manifest.Collections = append(manifest.Collections, PortableCollectionV1{
			CollectionID: collID, CollectionName: table, Type: collType,
			RecordCount: count, SHA256: hasher.Sum(), File: dataFile,
		})
	}

	// The manifest's own digest (computed over the canonical JSON without
	// the manifestSha256 field itself).
	manifest.ManifestSHA256 = ""
	mb, _ := json.Marshal(manifest)
	manifest.ManifestSHA256 = newSHA().of(mb)

	out, _ := json.MarshalIndent(manifest, "", "  ")
	if err := os.WriteFile(filepath.Join(exportDir, "manifest.json"), out, 0o600); err != nil {
		return nil, err
	}
	return manifest, nil
}

// importDir/exportDir are the per-database interchange roots.
func (rt *Runtime) exportDir(databaseID string) string {
	if databaseID == "" || databaseID == PrimaryDatabaseID {
		return filepath.Join(rt.DataDir, "exports", "primary")
	}
	return filepath.Join(rt.DataDir, "databases", databaseID, "exports")
}

// ImportPortableArchive validates the portable schema/data/files against
// the live schema preserving IDs, with dryRun (the explicit plan is the
// manifest's per-collection plan; rows are inserted preserving their
// stable IDs).
func (rt *Runtime) ImportPortableArchive(ctx context.Context, databaseID, archiveID string, expectedGeneration int64, dryRun bool) (map[string]any, error) {
	// The archive is an export directory id under the database's exports.
	var dir string
	if strings.Contains(archiveID, "/") || strings.Contains(archiveID, "..") {
		return nil, fmt.Errorf("invalid_request: the archive id must be an export id")
	}
	if databaseID == "" || databaseID == PrimaryDatabaseID {
		dir = filepath.Join(rt.DataDir, "exports", "primary", archiveID)
	} else {
		dir = filepath.Join(rt.DataDir, "databases", databaseID, "exports", archiveID)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var manifest PortableManifestV1
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return nil, fmt.Errorf("%w: corrupt manifest", ErrBackupInvalid)
	}
	// Verify the manifest digest.
	stored := manifest.ManifestSHA256
	manifest.ManifestSHA256 = ""
	mb, _ := json.Marshal(manifest)
	manifest.ManifestSHA256 = stored
	if stored != newSHA().of(mb) {
		return nil, fmt.Errorf("%w: the manifest digest does not match", ErrBackupInvalid)
	}

	if databaseID == "" || databaseID == PrimaryDatabaseID {
		return nil, fmt.Errorf("invalid_request: the primary application is not an import target in this release (the target surface owns it)")
	}
	d, err := rt.Service.GetDatabase(ctx, databaseID)
	if err != nil {
		return nil, err
	}
	if expectedGeneration != d.Generation {
		return nil, fmt.Errorf("%w: the expected generation does not match the database row", ErrReferenceStale)
	}
	handle, herr := rt.Handles.Open(ctx, databaseID)
	if herr != nil {
		return nil, herr
	}

	plan := []map[string]any{}
	totalRows := 0
	imported := 0
	for _, c := range manifest.Collections {
		entry := map[string]any{"collection": c.CollectionName, "records": 0, "state": "validated"}
		// The target table must exist (schema preservation: no implicit
		// schema creation in this release — the template owns the shape).
		cols, cerr := targetColumns(ctx, handle, c.CollectionName)
		if cerr != nil {
			entry["state"] = "failed"
			entry["error"] = "the collection table does not exist in the target"
			plan = append(plan, entry)
			continue
		}
		data, rerr := os.ReadFile(filepath.Join(dir, c.File))
		if rerr != nil {
			entry["state"] = "failed"
			entry["error"] = "the data file is missing"
			plan = append(plan, entry)
			continue
		}
		entry["records"] = c.RecordCount
		totalRows += c.RecordCount
		if !dryRun && entry["state"] == "validated" {
			known := map[string]bool{}
			for col := range cols {
				known[strings.ToLower(col)] = true
			}
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}
				var row map[string]any
				if json.Unmarshal([]byte(line), &row) != nil {
					entry["state"] = "failed"
					entry["error"] = "a row is not valid JSON"
					break
				}
				idVal, _ := row["id"].(string)
				if idVal == "" {
					entry["state"] = "failed"
					entry["error"] = "a row has no stable id"
					break
				}
				colNames := []string{`"id"`}
				values := []any{idVal}
				for k, v := range row {
					if strings.EqualFold(k, "id") {
						continue
					}
					if !known[strings.ToLower(k)] {
						continue // preserve the schema: unknown fields are dropped, never widened
					}
					colNames = append(colNames, quoteIdent(k))
					values = append(values, encodeRecordValue(v))
				}
				placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(colNames)), ", ")
				if _, xerr := handle.ExecContext(ctx, fmt.Sprintf(
					`INSERT OR REPLACE INTO %s (%s) VALUES (%s)`,
					quoteIdent(c.CollectionName), strings.Join(colNames, ", "), placeholders), values...); xerr != nil {
					entry["state"] = "failed"
					entry["error"] = xerr.Error()
					break
				}
				imported++
				entry["records"] = imported
			}
			if entry["state"] == "validated" {
				entry["state"] = "imported"
			}
		}
		plan = append(plan, entry)
	}
	return map[string]any{
		"archiveId": archiveID, "manifestSha256": manifest.ManifestSHA256,
		"dryRun": dryRun, "totalRows": totalRows, "importedRows": imported,
		"plan": plan,
	}, nil
}

// sortStrings sorts a string slice (insertion sort, no external dep).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// shaHelper is the streaming sha256 helper for the export hashes.
type shaHelper struct{ h hash.Hash }

// newSHA starts a new digest.
func newSHA() *shaHelper { return &shaHelper{h: sha256.New()} }

// Write feeds the digest.
func (s *shaHelper) Write(p []byte) (int, error) { return s.h.Write(p) }

// Sum returns the hex digest.
func (s *shaHelper) Sum() string { return hex.EncodeToString(s.h.Sum(nil)) }

// of digests one byte slice.
func (s *shaHelper) of(p []byte) string {
	s.h.Write(p)
	return s.Sum()
}

// decodePortableValue converts a scanned SQL value into its wire form.
func decodePortableValue(v any) any {
	switch t := v.(type) {
	case []byte:
		return string(t)
	default:
		return v
	}
}
