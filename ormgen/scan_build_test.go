package ormgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanPackage_whenBuildTaggedFileExcluded_shouldIgnoreInactiveFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeScanBuildFile(t, dir, "active.go", `package sample

//goark-orm:entity(table="sys_active")
type ActiveUser struct {
	ID int64 `+"`goark-orm:\"column='id';primary-key=true\"`"+`
}
`)
	writeScanBuildFile(t, dir, "inactive.go", `//go:build goark_orm_inactive

package sample

//goark-orm:entity(table="sys_inactive")
type InactiveUser struct {
	ID int64
}
`)

	model, err := ScanPackage(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("scan package failed: %v", err)
	}
	if len(model.Entities) != 1 || model.Entities[0].TypeName != "ActiveUser" {
		t.Fatalf("unexpected entities %#v", model.Entities)
	}
}

func TestScanPackage_whenBuildTagsProvided_shouldIncludeMatchingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeScanBuildFile(t, dir, "active.go", `package sample

//goark-orm:entity(table="sys_active")
type ActiveUser struct {
	ID int64 `+"`goark-orm:\"column='id';primary-key=true\"`"+`
}
`)
	writeScanBuildFile(t, dir, "tagged.go", `//go:build goark_orm_extra

package sample

//goark-orm:entity(table="sys_extra")
type ExtraUser struct {
	ID int64 `+"`goark-orm:\"column='id';primary-key=true\"`"+`
}
`)

	model, err := ScanPackage(GenerateSpec{Dir: dir, BuildTags: []string{"goark_orm_extra"}})
	if err != nil {
		t.Fatalf("scan package failed: %v", err)
	}
	if len(model.Entities) != 2 {
		t.Fatalf("unexpected entities %#v", model.Entities)
	}
	if model.Entities[0].TypeName != "ActiveUser" || model.Entities[1].TypeName != "ExtraUser" {
		t.Fatalf("unexpected entity order %#v", model.Entities)
	}
}

func writeScanBuildFile(t *testing.T, dir string, name string, source string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(source), 0o644); err != nil {
		t.Fatalf("write %s failed: %v", name, err)
	}
}
