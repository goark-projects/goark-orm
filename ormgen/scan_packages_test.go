package ormgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_whenMapperEmbedsImportedInterface_shouldFlattenTypedMethods(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/goark-orm-typed-scan\n\ngo 1.25\n")
	mustWriteFile(t, filepath.Join(dir, "shared", "reader.go"), `package shared

import ctxpkg "context"

type UserReadMapper interface {
	//goark-orm:select(sql="select count(1) from sys_user where id = #{id}")
	CountByID(ctx ctxpkg.Context, id int64) (int64, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "sample", "mapper.go"), `package sample

import (
	"context"

	"example.com/goark-orm-typed-scan/shared"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`goark-orm:\"column='id';primary-key=true\"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	shared.UserReadMapper

	//goark-orm:select(sql="select id from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: filepath.Join(dir, "sample")})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`func (m *goarkORMUserMapper) CountByID(ctx context.Context, id int64) (int64, error)`,
		`m.session.QueryOne(ctx, "system.user.UserMapper.CountByID", orm.NamedArgs{"_parameter": id, "id": id, "param1": id}, &out)`,
		`func (m *goarkORMUserMapper) FindByID(ctx context.Context, id int64) (*User, error)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenMapperUsesAliasedORMImport_shouldCanonicalizeGeneratedSignature(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/goark-orm-orm-alias\n\ngo 1.25\n\nrequire goark.dev/orm v0.0.0\n\nreplace goark.dev/orm => "+filepath.ToSlash(moduleRootForScanTest(t))+"\n")
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import (
	"context"

	ormx "goark.dev/orm"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`goark-orm:\"column='id';primary-key=true\"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id from sys_user where id = #{id}")
	ListPage(ctx context.Context, id int64, page ormx.PageRequest) (ormx.Page[User], error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`func (m *goarkORMUserMapper) ListPage(ctx context.Context, id int64, page orm.PageRequest) (orm.Page[User], error)`,
		`return orm.QueryPage[User](ctx, m.session, "system.user.UserMapper.ListPage", orm.NamedArgs{"_parameter": id, "id": id, "param1": id}, page)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func moduleRootForScanTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve module root failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("module root missing go.mod: %v", err)
	}
	return root
}
