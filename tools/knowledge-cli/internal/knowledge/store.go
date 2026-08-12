package knowledge

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrNotFound = errors.New("document not found")

type Store struct {
	root string
}

type Document struct {
	Path     string    `json:"path"`
	Category string    `json:"category"`
	Title    string    `json:"title"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	Content  string    `json:"content,omitempty"`
}

type Match struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

var reservedTopLevel = map[string]struct{}{
	".git":  {},
	"tools": {},
}

func NewStore(root string) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("knowledge root cannot be empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve knowledge root: %w", err)
	}
	clean := filepath.Clean(abs)
	if info, statErr := os.Lstat(clean); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("knowledge root cannot be a symbolic link")
	} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect knowledge root: %w", statErr)
	}
	return &Store{root: clean}, nil
}

func (s *Store) Root() string { return s.root }

func (s *Store) Init() error {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return fmt.Errorf("create knowledge root: %w", err)
	}
	return nil
}

func (s *Store) Categories() ([]string, error) {
	docs, err := s.List("")
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{})
	for _, doc := range docs {
		if doc.Category != "" {
			set[doc.Category] = struct{}{}
		}
	}
	categories := make([]string, 0, len(set))
	for category := range set {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	return categories, nil
}

func (s *Store) List(category string) ([]Document, error) {
	base := s.root
	if category != "" {
		var err error
		base, err = s.resolveDirectory(category)
		if err != nil {
			return nil, err
		}
	}
	if info, err := os.Stat(base); errors.Is(err, fs.ErrNotExist) {
		return []Document{}, nil
	} else if err != nil {
		return nil, fmt.Errorf("inspect knowledge root: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("category is not a directory: %s", category)
	}

	docs := make([]Document, 0)
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() && path != base && s.isReserved(path) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !isDocumentExtension(filepath.Ext(entry.Name())) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		doc, err := s.metadata(path, info)
		if err != nil {
			return err
		}
		docs = append(docs, doc)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	return docs, nil
}

func (s *Store) Read(name string) (Document, error) {
	path, err := s.resolveDocument(name)
	if err != nil {
		return Document{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, fmt.Errorf("read document: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return Document{}, fmt.Errorf("inspect document: %w", err)
	}
	doc, err := s.metadata(path, info)
	if err != nil {
		return Document{}, err
	}
	doc.Content = string(data)
	return doc, nil
}

func (s *Store) Write(name, content, mode string) (Document, error) {
	path, err := s.resolveDocument(name)
	if err != nil {
		return Document{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return Document{}, fmt.Errorf("create category: %w", err)
	}
	if err := s.rejectSymlinkComponents(path); err != nil {
		return Document{}, err
	}

	flags := 0
	switch mode {
	case "create", "":
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	case "overwrite":
		flags = os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	case "append":
		flags = os.O_WRONLY | os.O_CREATE | os.O_APPEND
	default:
		return Document{}, fmt.Errorf("invalid write mode %q (use create, overwrite, or append)", mode)
	}

	file, err := os.OpenFile(path, flags, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return Document{}, fmt.Errorf("document already exists: %s", s.relative(path))
	}
	if err != nil {
		return Document{}, fmt.Errorf("open document for writing: %w", err)
	}
	if _, err = file.WriteString(content); err != nil {
		_ = file.Close()
		return Document{}, fmt.Errorf("write document: %w", err)
	}
	if err = file.Close(); err != nil {
		return Document{}, fmt.Errorf("close document: %w", err)
	}
	return s.Read(name)
}

func (s *Store) Search(query, category string, limit int) ([]Match, error) {
	if query == "" {
		return nil, errors.New("search query cannot be empty")
	}
	docs, err := s.List(category)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(query)
	matches := make([]Match, 0)
	for _, doc := range docs {
		path, err := s.resolveDocument(doc.Path)
		if err != nil {
			return nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("search %s: %w", doc.Path, err)
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		line := 0
		for scanner.Scan() {
			line++
			if strings.Contains(strings.ToLower(scanner.Text()), needle) {
				matches = append(matches, Match{Path: doc.Path, Line: line, Text: scanner.Text()})
				if limit > 0 && len(matches) >= limit {
					_ = file.Close()
					return matches, nil
				}
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if scanErr != nil {
			return nil, fmt.Errorf("search %s: %w", doc.Path, scanErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", doc.Path, closeErr)
		}
	}
	return matches, nil
}

func (s *Store) resolveDocument(name string) (string, error) {
	clean, err := cleanRelative(name)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(clean)
	if ext == "" {
		clean += ".md"
	} else if !isDocumentExtension(ext) {
		return "", errors.New("documents must use .md or .markdown")
	}
	path := filepath.Join(s.root, clean)
	if err := s.rejectSymlinkComponents(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) resolveDirectory(name string) (string, error) {
	clean, err := cleanRelative(name)
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.root, clean)
	if err := s.rejectSymlinkComponents(path); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Store) rejectSymlinkComponents(path string) error {
	rel, err := filepath.Rel(s.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("path escapes knowledge root")
	}
	current := s.root
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symbolic links are not allowed in knowledge paths")
		}
	}
	return nil
}

func (s *Store) metadata(path string, info fs.FileInfo) (Document, error) {
	rel := s.relative(path)
	title, err := readTitle(path)
	if err != nil {
		return Document{}, err
	}
	category := filepath.ToSlash(filepath.Dir(rel))
	if category == "." {
		category = ""
	}
	return Document{Path: rel, Category: category, Title: title, Size: info.Size(), Modified: info.ModTime()}, nil
}

func (s *Store) relative(path string) string {
	rel, _ := filepath.Rel(s.root, path)
	return filepath.ToSlash(rel)
}

func (s *Store) isReserved(path string) bool {
	rel, err := filepath.Rel(s.root, path)
	if err != nil || rel == "." {
		return false
	}
	top := strings.Split(rel, string(filepath.Separator))[0]
	_, reserved := reservedTopLevel[strings.ToLower(top)]
	return reserved
}

func readTitle(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read title: %w", err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "# ")), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read title: %w", err)
	}
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)), nil
}

func cleanRelative(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("document path cannot be empty")
	}
	if filepath.IsAbs(name) || filepath.VolumeName(name) != "" || os.IsPathSeparator(name[0]) {
		return "", errors.New("absolute paths are not allowed")
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("parent directory traversal is not allowed")
	}
	top := strings.Split(clean, string(filepath.Separator))[0]
	if _, reserved := reservedTopLevel[strings.ToLower(top)]; reserved {
		return "", fmt.Errorf("%s is a reserved top-level path", top)
	}
	return clean, nil
}

func isDocumentExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".md", ".markdown":
		return true
	default:
		return false
	}
}
