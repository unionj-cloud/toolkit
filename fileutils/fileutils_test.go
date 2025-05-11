package fileutils

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mholt/archives"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unionj-cloud/toolkit/pathutils"
)

func TestCreateDirectory(t *testing.T) {
	type args struct {
		dir string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "",
			args: args{
				dir: pathutils.Abs("testfiles"),
			},
			// it should have error because testfiles has already existed as a file not a directory
			wantErr: true,
		},
		{
			name: "",
			args: args{
				dir: "/TestCreateDirectory",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CreateDirectory(tt.args.dir); (err != nil) != tt.wantErr {
				t.Errorf("CreateDirectory() error = %v, wantErr %v", err, tt.wantErr)
			}
			//defer os.RemoveAll(dir)
		})
	}
}

func TestArchive(t *testing.T) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "archive-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// 创建测试文件
	testFile1 := filepath.Join(tempDir, "test1.txt")
	err = os.WriteFile(testFile1, []byte("测试内容1"), 0644)
	require.NoError(t, err)

	// 创建测试目录及其中的文件
	testSubDir := filepath.Join(tempDir, "subdir")
	err = os.Mkdir(testSubDir, 0755)
	require.NoError(t, err)

	testFile2 := filepath.Join(testSubDir, "test2.txt")
	err = os.WriteFile(testFile2, []byte("测试内容2"), 0644)
	require.NoError(t, err)

	tests := []struct {
		name       string
		outputName string
	}{
		{
			name:       "Zip归档",
			outputName: "archive.zip",
		},
		{
			name:       "Tar归档",
			outputName: "archive.tar",
		},
		{
			name:       "TarGz归档",
			outputName: "archive.tar.gz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置输出文件路径
			outputPath := filepath.Join(tempDir, tt.outputName)

			// 调用Archive函数
			err := Archive(outputPath, testFile1, testSubDir)
			require.NoError(t, err)

			// 验证归档文件存在
			_, err = os.Stat(outputPath)
			assert.NoError(t, err)

			// 验证归档文件内容
			validateArchive(t, outputPath, []string{"test1.txt", "subdir/test2.txt"})
		})
	}
}

// 验证归档文件内容
func validateArchive(t *testing.T, archivePath string, expectedPaths []string) {
	// 打开归档文件
	f, err := os.Open(archivePath)
	require.NoError(t, err)
	defer f.Close()

	// 识别归档格式
	format, reader, err := archives.Identify(context.Background(), archivePath, f)
	require.NoError(t, err)

	extractor, ok := format.(archives.Extractor)
	require.True(t, ok, "格式无法用于提取")

	// 提取文件并验证
	foundPaths := make(map[string]bool)
	err = extractor.Extract(context.Background(), reader, func(ctx context.Context, file archives.FileInfo) error {
		if !file.IsDir() {
			foundPaths[file.NameInArchive] = true
		}
		return nil
	})
	require.NoError(t, err)

	// 检查是否找到所有期望的文件
	for _, expectedPath := range expectedPaths {
		assert.True(t, foundPaths[expectedPath], "在归档中未找到文件: %s", expectedPath)
	}
}
