package fileutils

import (
	"bufio"
	"context"
	"io"
	"os"

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
	ctx := context.Background()

	// 通过文件扩展名自动识别格式
	format, _, err := archives.Identify(ctx, output, nil)
	if err != nil {
		return err
	}

	archiver, ok := format.(archives.Archiver)
	if !ok {
		return errors.New("不支持的归档格式")
	}

	// 创建输出文件
	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	// 准备源文件映射和文件列表
	filesMap := make(map[string]string)
	for _, source := range sources {
		filesMap[source] = ""
	}

	files, err := archives.FilesFromDisk(ctx, nil, filesMap)
	if err != nil {
		return err
	}

	// 创建归档
	return archiver.Archive(ctx, out, files)
}
