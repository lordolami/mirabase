// Embedded MiraBase example used by the AT-CORE acceptance harness.
//
// It bootstraps a MiraBase application in-process, provisions a "posts"
// collection identical to the one used by the ready-made executable
// harness, exercises the full record CRUD cycle (including a validation
// negative control and a restart persistence check) and prints a
// deterministic transcript of every step.
//
// Usage:
//
//	go run ./examples/embedded --dir=/path/to/data/dir
package main

import (
	"flag"
	"fmt"
	"os"

	"mirabase"
	"mirabase/core"
)

func main() {
	dataDir := flag.String("dir", "", "the app data directory")
	flag.Parse()

	if *dataDir == "" {
		fmt.Println("FATAL: --dir is required")
		os.Exit(1)
	}

	if err := run(*dataDir); err != nil {
		fmt.Printf("FATAL: %v\n", err)
		os.Exit(1)
	}
}

func run(dataDir string) error {
	if err := engineEnsure(); err != nil {
		return fmt.Errorf("engine: %w", err)
	}

	app := mirabase.NewWithConfig(mirabase.Config{
		DefaultDataDir: dataDir,
		DBConnect:      dbConnect(),
	})
	defer app.ClearBootstrap()

	if err := app.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	// ensure the system collections are present (sanity check)
	if _, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers); err != nil {
		return fmt.Errorf("missing %s collection: %w", core.CollectionNameSuperusers, err)
	}

	collection, err := ensurePostsCollection(app)
	if err != nil {
		return err
	}

	// CREATE
	record := core.NewRecord(collection)
	record.Set("title", "hello world")
	if err := app.Save(record); err != nil {
		return fmt.Errorf("create record: %w", err)
	}
	recordId := record.Id
	fmt.Printf("CREATE: id=%s title=%s\n", recordId, record.GetString("title"))

	// READ
	stored, err := app.FindRecordById(collection.Name, recordId)
	if err != nil {
		return fmt.Errorf("read record: %w", err)
	}
	fmt.Printf("READ: id=%s title=%s\n", stored.Id, stored.GetString("title"))

	// UPDATE
	stored.Set("title", "hello mirabase")
	if err := app.Save(stored); err != nil {
		return fmt.Errorf("update record: %w", err)
	}
	fmt.Printf("UPDATE: id=%s title=%s\n", stored.Id, stored.GetString("title"))

	// NEGATIVE CONTROL: required "title" validation must fail
	invalid := core.NewRecord(collection)
	invalid.Set("title", "")
	if err := app.Save(invalid); err == nil {
		return fmt.Errorf("negative control failed: expected a validation error for an empty title")
	} else {
		fmt.Printf("NEGATIVE: empty title rejected with: %v\n", firstErrorLine(err))
	}

	// RESTART: simulate a full app restart over the same data directory
	if err := app.ClearBootstrap(); err != nil {
		return fmt.Errorf("shutdown for restart: %w", err)
	}

	restarted := mirabase.NewWithConfig(mirabase.Config{
		DefaultDataDir: dataDir,
		DBConnect:      dbConnect(),
	})
	defer restarted.ClearBootstrap()
	if err := restarted.Bootstrap(); err != nil {
		return fmt.Errorf("bootstrap after restart: %w", err)
	}

	persisted, err := restarted.FindRecordById(collection.Name, recordId)
	if err != nil {
		return fmt.Errorf("read record after restart: %w", err)
	}
	fmt.Printf("RESTART: id=%s title=%s\n", persisted.Id, persisted.GetString("title"))

	// DELETE
	if err := restarted.Delete(persisted); err != nil {
		return fmt.Errorf("delete record: %w", err)
	}
	count, err := restarted.CountRecords(collection.Name)
	if err != nil {
		return fmt.Errorf("count records: %w", err)
	}
	if count != 0 {
		return fmt.Errorf("delete verification failed: expected 0 records, got %d", count)
	}
	fmt.Printf("DELETE: id=%s remaining=%d\n", persisted.Id, count)

	fmt.Println("RESULT: OK")
	return nil
}

func ensurePostsCollection(app core.App) (*core.Collection, error) {
	existing, err := app.FindCollectionByNameOrId("posts")
	if err == nil {
		return existing, nil
	}

	collection := core.NewCollection(core.CollectionTypeBase, "posts")
	collection.Fields.Add(&core.TextField{
		Name:     "title",
		Required: true,
		Min:      1,
		Max:      255,
	})

	if err := app.Save(collection); err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	fmt.Printf("COLLECTION: created posts (type=base)\n")

	return collection, nil
}

func firstErrorLine(err error) string {
	msg := err.Error()
	for i, r := range msg {
		if r == '\n' {
			return msg[:i]
		}
	}
	return msg
}
