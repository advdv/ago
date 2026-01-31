// Package dirhash provides content-based hashing of directories
// with support for dockerignore-style pattern filtering.
package dirhash

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/moby/patternmatcher"
)

// IgnoreParser parses ignore patterns from a source.
type IgnoreParser interface {
	Parse(r io.Reader) ([]string, error)
}

// PatternMatcher determines if a path should be excluded.
type PatternMatcher interface {
	Match(path string, parentInfo any) (matched bool, info any, err error)
	HasNegation() bool
}

// FileReader reads file contents for hashing.
type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

// Logger receives debug output during hashing.
type Logger interface {
	LogFile(path string, reason string)
	LogDir(path string)
	LogSkip(path string, isDir bool)
}

// Hasher computes a content-based hash of a directory.
type Hasher struct {
	ignoreParser   IgnoreParser
	fileReader     FileReader
	logger         Logger
	alwaysInclude  map[string]bool
	truncateLength int
}

// Option configures a Hasher.
type Option func(*Hasher)

// WithIgnoreParser sets a custom ignore pattern parser.
func WithIgnoreParser(p IgnoreParser) Option {
	return func(h *Hasher) {
		h.ignoreParser = p
	}
}

// WithFileReader sets a custom file reader.
func WithFileReader(r FileReader) Option {
	return func(h *Hasher) {
		h.fileReader = r
	}
}

// WithLogger sets a logger for debug output.
func WithLogger(l Logger) Option {
	return func(h *Hasher) {
		h.logger = l
	}
}

// WithAlwaysInclude sets paths that are always included regardless of ignore patterns.
func WithAlwaysInclude(paths ...string) Option {
	return func(h *Hasher) {
		h.alwaysInclude = make(map[string]bool, len(paths))
		for _, p := range paths {
			h.alwaysInclude[p] = true
		}
	}
}

// WithTruncateLength sets the hash output length (0 for full hash).
func WithTruncateLength(n int) Option {
	return func(h *Hasher) {
		h.truncateLength = n
	}
}

// New creates a new Hasher with the given options.
func New(opts ...Option) *Hasher {
	h := &Hasher{
		ignoreParser:   &dockerignoreParser{},
		fileReader:     &osFileReader{},
		logger:         &nullLogger{},
		alwaysInclude:  map[string]bool{},
		truncateLength: 12,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Hash computes the content hash of a directory.
// It reads the ignore file (e.g., .dockerignore) from the directory root.
func (h *Hasher) Hash(dir string, ignoreFileName string) (string, error) {
	matcher, err := h.loadIgnorePatterns(dir, ignoreFileName)
	if err != nil {
		return "", err
	}

	files, err := h.collectFiles(dir, matcher)
	if err != nil {
		return "", err
	}

	return h.hashFiles(dir, files)
}

// CollectedFiles returns the list of files that would be hashed (for testing/debugging).
func (h *Hasher) CollectedFiles(dir string, ignoreFileName string) ([]string, error) {
	matcher, err := h.loadIgnorePatterns(dir, ignoreFileName)
	if err != nil {
		return nil, err
	}

	return h.collectFiles(dir, matcher)
}

func (h *Hasher) loadIgnorePatterns(dir, ignoreFileName string) (*mobyMatcher, error) {
	ignorePath := filepath.Join(dir, ignoreFileName)

	f, err := os.Open(ignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			pm, err := patternmatcher.New(nil)
			if err != nil {
				return nil, err
			}
			return &mobyMatcher{pm: pm, hasNegation: false}, nil
		}
		return nil, errors.Wrapf(err, "failed to open %s", ignoreFileName)
	}
	defer f.Close()

	patterns, err := h.ignoreParser.Parse(f)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse %s", ignoreFileName)
	}

	hasNegation := false
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			hasNegation = true
			break
		}
	}

	pm, err := patternmatcher.New(patterns)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile patterns from %s", ignoreFileName)
	}

	return &mobyMatcher{pm: pm, hasNegation: hasNegation}, nil
}

func (h *Hasher) collectFiles(dir string, matcher *mobyMatcher) ([]string, error) {
	parentMatchInfo := make(map[string]patternmatcher.MatchInfo)
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		relPath = filepath.ToSlash(relPath)
		isDir := d.IsDir()

		if h.alwaysInclude[relPath] {
			if !isDir {
				h.logger.LogFile(relPath, "always included")
				files = append(files, relPath)
			}
			return nil
		}

		parentPath := filepath.Dir(relPath)
		if parentPath == "." {
			parentPath = ""
		}

		var parentInfo patternmatcher.MatchInfo
		if parentPath != "" {
			parentInfo = parentMatchInfo[parentPath]
		}

		matched, matchInfo, err := matcher.pm.MatchesUsingParentResults(relPath, parentInfo)
		if err != nil {
			return errors.Wrapf(err, "pattern match failed for %s", relPath)
		}

		if isDir {
			parentMatchInfo[relPath] = matchInfo
			if matched && !matcher.hasNegation {
				h.logger.LogSkip(relPath, true)
				return filepath.SkipDir
			}
			h.logger.LogDir(relPath)
			return nil
		}

		if matched {
			h.logger.LogSkip(relPath, false)
			return nil
		}

		h.logger.LogFile(relPath, "")
		files = append(files, relPath)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to walk directory")
	}

	sort.Strings(files)
	return files, nil
}

func (h *Hasher) hashFiles(dir string, files []string) (string, error) {
	hash := sha256.New()

	for _, relPath := range files {
		absPath := filepath.Join(dir, relPath)

		content, err := h.fileReader.ReadFile(absPath)
		if err != nil {
			return "", errors.Wrapf(err, "failed to read %s", relPath)
		}

		hash.Write([]byte(relPath))
		hash.Write([]byte{0})
		hash.Write(content)
	}

	fullHash := fmt.Sprintf("%x", hash.Sum(nil))
	if h.truncateLength > 0 && len(fullHash) > h.truncateLength {
		return fullHash[:h.truncateLength], nil
	}
	return fullHash, nil
}

// mobyMatcher wraps patternmatcher.PatternMatcher.
type mobyMatcher struct {
	pm          *patternmatcher.PatternMatcher
	hasNegation bool
}

// dockerignoreParser parses .dockerignore-style files.
type dockerignoreParser struct{}

func (p *dockerignoreParser) Parse(r io.Reader) ([]string, error) {
	var patterns []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return patterns, nil
}

// osFileReader reads files from the OS filesystem.
type osFileReader struct{}

func (r *osFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// nullLogger discards all log messages.
type nullLogger struct{}

func (l *nullLogger) LogFile(path string, reason string) {}
func (l *nullLogger) LogDir(path string)                 {}
func (l *nullLogger) LogSkip(path string, isDir bool)    {}

// DebugLogger writes debug output to a writer.
type DebugLogger struct {
	W io.Writer
}

// LogFile logs an included file.
func (l *DebugLogger) LogFile(path string, reason string) {
	if reason != "" {
		fmt.Fprintf(l.W, "FILE: %s (%s)\n", path, reason)
	} else {
		fmt.Fprintf(l.W, "FILE: %s\n", path)
	}
}

// LogDir logs a traversed directory.
func (l *DebugLogger) LogDir(path string) {
	fmt.Fprintf(l.W, "DIR:  %s/\n", path)
}

// LogSkip logs a skipped path.
func (l *DebugLogger) LogSkip(path string, isDir bool) {
	if isDir {
		fmt.Fprintf(l.W, "SKIP: %s/\n", path)
	} else {
		fmt.Fprintf(l.W, "SKIP: %s\n", path)
	}
}
