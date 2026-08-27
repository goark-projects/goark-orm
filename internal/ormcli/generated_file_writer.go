package ormcli

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// generatedFileWriteResult 描述一次生成文件写入是否真正改变磁盘内容。
type generatedFileWriteResult struct {
	Path    string
	Changed bool
}

// writeGeneratedFile 原子替换生成文件；内容一致时跳过写入以减少无效构建变更。
func writeGeneratedFile(path string, data []byte) (generatedFileWriteResult, error) {
	path = filepath.Clean(path)
	result := generatedFileWriteResult{Path: path}
	current, err := os.ReadFile(path)
	switch {
	case err == nil && bytes.Equal(current, data):
		return result, nil
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
	default:
		return result, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return result, err
	}
	mode := generatedFileMode(path)
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return result, err
	}
	tempName := temp.Name()
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tempName)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return result, err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return result, err
	}
	if err := temp.Close(); err != nil {
		return result, err
	}
	if err := os.Chmod(tempName, mode); err != nil {
		return result, err
	}
	if err := os.Rename(tempName, path); err != nil {
		return result, err
	}
	renamed = true
	result.Changed = true
	return result, nil
}

func generatedFileMode(path string) fs.FileMode {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return 0o644
	}
	return info.Mode().Perm()
}
