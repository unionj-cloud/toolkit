package fileutils

import (
	"bufio"
	"context"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archives"
	"github.com/pkg/errors"
)

// CreateDirectory dir didn't exist, then create dir, otherwise do nothing.
func CreateDirectory(dir string) (err error) {
	var info os.FileInfo
	if info, err = os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(dir, 0755); err != nil {
				return
			}
		}
	} else {
		if !info.IsDir() {
			return errors.New("not a directory: " + dir)
		}
	}
	return
}

func File2lines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return LinesFromReader(f)
}

func LinesFromReader(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

func Archive(output string, sources ...string) error {
	if len(sources) == 0 {
		return stderrors.New("no sources")
	}
	ctx := context.Background()
	format, _, err := archives.Identify(ctx, filepath.Base(output), nil)
	if err != nil {
		if stderrors.Is(err, archives.NoMatch) {
			return stderrors.New("unsupported archive extension: " + output)
		}
		return err
	}
	aw, ok := format.(archives.Archiver)
	if !ok {
		return stderrors.New("format is not archivable: " + output)
	}
	m := make(map[string]string, len(sources))
	for _, src := range sources {
		abs, err := filepath.Abs(src)
		if err != nil {
			return err
		}
		m[abs] = ""
	}
	files, err := archives.FilesFromDisk(ctx, nil, m)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(output); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, dir)
		}
	}
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()
	return aw.Archive(ctx, out, files)
}
