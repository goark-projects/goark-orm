package ormcli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type generateORMMode int

const (
	generateORMModeWrite generateORMMode = iota
	generateORMModeDryRun
	generateORMModeValidate
	generateORMModeCheck
	generateORMModeDiff
)

func generateORMModeFromFlags(dryRun bool, validate bool, check bool, diff bool) (generateORMMode, error) {
	enabled := 0
	if dryRun {
		enabled++
	}
	if validate {
		enabled++
	}
	if check {
		enabled++
	}
	if diff {
		enabled++
	}
	if enabled > 1 {
		return generateORMModeWrite, fmt.Errorf("--dry-run、--validate、--check 和 --diff 不能同时使用")
	}
	switch {
	case dryRun:
		return generateORMModeDryRun, nil
	case validate:
		return generateORMModeValidate, nil
	case check:
		return generateORMModeCheck, nil
	case diff:
		return generateORMModeDiff, nil
	default:
		return generateORMModeWrite, nil
	}
}

func (m generateORMMode) requiresGeneratedFile() bool {
	return m == generateORMModeCheck || m == generateORMModeDiff
}

func (c Command) finishGeneratedFile(output string, source []byte, mode generateORMMode) int {
	output = filepath.Clean(output)
	switch mode {
	case generateORMModeDryRun:
		_, _ = fmt.Fprintf(c.Err, "dry-run generated %s\n", output)
		return 0
	case generateORMModeValidate:
		_, _ = fmt.Fprintf(c.Err, "validated %s\n", output)
		return 0
	case generateORMModeCheck:
		return c.checkGeneratedFile(output, source)
	case generateORMModeDiff:
		return c.diffGeneratedFile(output, source)
	default:
		if err := writeFile(output, source); err != nil {
			_, _ = fmt.Fprintf(c.Err, "写入生成文件失败: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(c.Err, "generated %s\n", output)
		return 0
	}
}

func (c Command) checkGeneratedFile(output string, source []byte) int {
	current, err := os.ReadFile(output)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, _ = fmt.Fprintf(c.Err, "generated file missing: %s\n", output)
			return 1
		}
		_, _ = fmt.Fprintf(c.Err, "读取生成文件失败: %v\n", err)
		return 1
	}
	if !bytes.Equal(current, source) {
		_, _ = fmt.Fprintf(c.Err, "generated file out of date: %s\n", output)
		return 1
	}
	_, _ = fmt.Fprintf(c.Err, "checked %s\n", output)
	return 0
}

func (c Command) diffGeneratedFile(output string, source []byte) int {
	current, err := os.ReadFile(output)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, _ = c.Out.Write([]byte(unifiedGeneratedDiff(output, nil, source)))
			return 1
		}
		_, _ = fmt.Fprintf(c.Err, "读取生成文件失败: %v\n", err)
		return 1
	}
	if bytes.Equal(current, source) {
		_, _ = fmt.Fprintf(c.Err, "checked %s\n", output)
		return 0
	}
	_, _ = c.Out.Write([]byte(unifiedGeneratedDiff(output, current, source)))
	return 1
}

func unifiedGeneratedDiff(path string, current []byte, expected []byte) string {
	var builder strings.Builder
	builder.WriteString("--- ")
	builder.WriteString(path)
	builder.WriteByte('\n')
	builder.WriteString("+++ generated")
	builder.WriteByte('\n')
	builder.WriteString("@@\n")
	for _, line := range splitDiffLines(string(current)) {
		builder.WriteByte('-')
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	for _, line := range splitDiffLines(string(expected)) {
		builder.WriteByte('+')
		builder.WriteString(line)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func splitDiffLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.TrimSuffix(value, "\n")
	if value == "" {
		return nil
	}
	return strings.Split(value, "\n")
}
