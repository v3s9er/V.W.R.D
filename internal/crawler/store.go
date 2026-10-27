package crawler

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Store struct {
	Root     string
	mu       sync.Mutex
	seen     map[string]string
	pathUses map[string]int
}

func NewStore(outputDir string) (*Store, error) {
	name := "capture-" + time.Now().Format("20060102-150405.000000000")
	root := filepath.Join(outputDir, name)
	if err := os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &Store{Root: root, seen: make(map[string]string), pathUses: make(map[string]int)}, nil
}

func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}

func (s *Store) SaveBody(rawURL, mime string, body []byte) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sum := sha256.Sum256(body)
	identity := rawURL + "|" + hex.EncodeToString(sum[:])
	if existing, ok := s.seen[identity]; ok {
		return existing, false, nil
	}
	base := sitePath(rawURL, mime)
	count := s.pathUses[base] + 1
	s.pathUses[base] = count
	rel := base
	if count > 1 {
		ext := filepath.Ext(base)
		rel = strings.TrimSuffix(base, ext) + "__v" + shortHash(identity) + ext
	}
	if err := s.write(rel, body); err != nil {
		return "", false, err
	}
	s.seen[identity] = filepath.ToSlash(rel)
	return filepath.ToSlash(rel), true, nil
}

func (s *Store) SavePage(page Page) (string, error) {
	parts := []string{"menus"}
	for _, part := range page.Menu.Path {
		parts = append(parts, safePart(part))
	}
	if len(parts) == 1 {
		parts = append(parts, "Home")
	}
	parts[len(parts)-1] += "__" + shortHash(page.Menu.URL)
	rel := filepath.Join(append(parts, "page.json")...)
	data, err := json.MarshalIndent(page, "", "  ")
	if err != nil {
		return "", err
	}
	if err := s.write(rel, data); err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

func (s *Store) SaveReport(report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return s.write("capture-report.json", data)
}

func (s *Store) write(rel string, data []byte) error {
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(filepath.Join(root, rel))
	if err != nil {
		return err
	}
	within, err := filepath.Rel(root, target)
	if err != nil || within == ".." || strings.HasPrefix(within, ".."+string(filepath.Separator)) {
		return errors.New("output path escaped capture directory")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, data, 0644)
}

func (s *Store) Zip() (string, error) {
	zipPath := s.Root + ".zip"
	out, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	writer := zip.NewWriter(out)
	err = filepath.WalkDir(s.Root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return errors.New("symbolic link in capture directory")
		}
		rel, err := filepath.Rel(s.Root, file)
		if err != nil {
			return err
		}
		input, err := os.Open(file)
		if err != nil {
			return err
		}
		entryWriter, err := writer.Create(filepath.ToSlash(rel))
		if err != nil {
			input.Close()
			return err
		}
		_, err = io.Copy(entryWriter, input)
		closeErr := input.Close()
		if err == nil {
			err = closeErr
		}
		return err
	})
	closeErr := writer.Close()
	fileCloseErr := out.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	if fileCloseErr != nil {
		return "", fileCloseErr
	}
	return zipPath, nil
}
