package knowledge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteReadListAndCategories(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Write("engineering/go/testing", "# Go testing\n\nUse table tests.\n", "create")
	if err != nil {
		t.Fatal(err)
	}
	if first.Path != "engineering/go/testing.md" || first.Title != "Go testing" {
		t.Fatalf("unexpected metadata: %#v", first)
	}
	if _, err := store.Write("engineering/go/testing", "duplicate", "create"); err == nil {
		t.Fatal("create should reject an existing document")
	}
	if _, err := store.Write("notes/agents", "# Agents\n", "overwrite"); err != nil {
		t.Fatal(err)
	}

	doc, err := store.Read("engineering/go/testing")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Content != "# Go testing\n\nUse table tests.\n" {
		t.Fatalf("unexpected content %q", doc.Content)
	}
	docs, err := store.List("engineering")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents", len(docs))
	}
	categories, err := store.Categories()
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 2 || categories[0] != "engineering/go" || categories[1] != "notes" {
		t.Fatalf("unexpected categories: %#v", categories)
	}
}

func TestAppendAndSearch(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	if _, err := store.Write("go/errors", "# Errors\nWrap errors.\n", "create"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Write("go/errors", "Match ERRORS with errors.Is.\n", "append"); err != nil {
		t.Fatal(err)
	}
	matches, err := store.Search("wrap errors", "go", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Line != 2 {
		t.Fatalf("unexpected matches: %#v", matches)
	}
}

func TestRejectsUnsafePathsAndExtensions(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	bad := []string{"../secret", ".git/config", "tools/source", "note.txt", filepath.Join(string(filepath.Separator), "absolute.md")}
	for _, name := range bad {
		if _, err := store.Write(name, "x", "create"); err == nil {
			t.Errorf("expected %q to be rejected", name)
		}
	}
	if _, err := store.Read("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListSkipsToolingDirectory(t *testing.T) {
	root := t.TempDir()
	store, _ := NewStore(root)
	if err := os.MkdirAll(filepath.Join(root, "tools", "knowledge-cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tools", "knowledge-cli", "README.md"), []byte("# CLI"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Knowledge"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 || docs[0].Path != "README.md" {
		t.Fatalf("unexpected documents: %#v", docs)
	}
}

func TestListMissingRootIsEmpty(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), "missing"))
	docs, err := store.List("")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected empty list, got %#v", docs)
	}
}

func TestTitleFallsBackToFilename(t *testing.T) {
	root := t.TempDir()
	store, _ := NewStore(root)
	if err := os.WriteFile(filepath.Join(root, "plain.md"), []byte("No heading."), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := store.Read("plain")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Title != "plain" {
		t.Fatalf("unexpected title %q", doc.Title)
	}
}
