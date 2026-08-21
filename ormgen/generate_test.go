package ormgen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerate_whenXMLAndAnnotationMixed_shouldRenderMetadataAndMapperImpl(t *testing.T) {
	dir := writeSamplePackage(t)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		"package sample",
		"func RegisterGoarkORMMetadata(registry *orm.Registry) error",
		"TypeName: \"User\"",
		"ColumnName: \"id\"",
		"Namespace: \"system.user.UserMapper\"",
		"Source: orm.StatementSource(\"xml\")",
		"Source: orm.StatementSource(\"annotation\")",
		"type goarkORMUserMapper struct",
		"func NewUserMapper(session orm.Session) UserMapper",
		"func (m *goarkORMUserMapper) FindByID(ctx context.Context, id int64) (*User, error)",
		"m.session.QueryOne(ctx, \"system.user.UserMapper.FindByID\", orm.NamedArgs{\"id\": id}, &out)",
		"func (m *goarkORMUserMapper) ListByStatus(ctx context.Context, status string) ([]User, error)",
		"m.session.Query(ctx, \"system.user.UserMapper.ListByStatus\", orm.NamedArgs{\"status\": status}, &out)",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenGeneratedSourceCompiledInTempModule_shouldPassGoTest(t *testing.T) {
	dir := writeSamplePackage(t)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	mustWriteFile(t, filepath.Join(dir, "zz_goark_orm_sample_gen.go"), string(source))
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolve module root failed: %v", err)
	}
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/goark-orm-smoke\n\ngo 1.25\n\nrequire goark.dev/orm v0.0.0\n\nreplace goark.dev/orm => "+filepath.ToSlash(root)+"\n")

	cmd := exec.Command("go", "test", "-mod=mod", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated source did not compile: %v\n%s", err, output)
	}
}

func TestGenerate_whenAnnotationAndXMLBindSameMethod_shouldReject(t *testing.T) {
	dir := writeSamplePackage(t)
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true;auto-increment=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	Status string `+"`"+`goark-orm:"column='status'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	//goark-orm:select(sql="select id, name, status from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "declared by both XML and annotation") {
		t.Fatalf("expected duplicate statement error, got %v", err)
	}
}

func TestGenerate_whenMapperNamespaceMissing_shouldReject(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper()
type UserMapper interface {
	//goark-orm:select(sql="select id from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "missing required namespace") {
		t.Fatalf("expected namespace error, got %v", err)
	}
}

func TestGenerate_whenXMLNamespaceMismatch_shouldReject(t *testing.T) {
	dir := writeSamplePackage(t)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.OtherMapper">
  <select id="FindByID" resultMap="UserResult">
    select id, name, status from sys_user where id = #{id}
  </select>
</mapper>
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "does not match mapper namespace") {
		t.Fatalf("expected namespace mismatch error, got %v", err)
	}
}

func TestGenerate_whenXMLUsesCDATA_shouldRenderNormalizedSQL(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByName(ctx context.Context, name string) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <resultMap id="UserResult" type="User">
    <id property="ID" column="id"/>
    <result property="Name" column="name"/>
  </resultMap>
  <select id="FindByName" resultMap="UserResult"><![CDATA[
    select id, name
    from sys_user
    where name <> #{name}
  ]]></select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	if !strings.Contains(output, `SQL: "select id, name\nfrom sys_user\nwhere name <> #{name}"`) {
		t.Fatalf("expected CDATA SQL to be normalized, got:\n%s", output)
	}
}

func TestGenerate_whenXMLContainsDynamicElement_shouldRenderDynamicSQLMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Status string `+"`"+`goark-orm:"column='status'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	List(ctx context.Context, status string) ([]User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="List">
    select id, status
    from sys_user
    <where>
      <if test="status != nil and status != ''">
        status = #{status}
      </if>
    </where>
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		"DynamicSQL: []orm.DynamicSQLNode{",
		"Kind: orm.DynamicSQLNodeWhere",
		"Kind: orm.DynamicSQLNodeIf",
		"Test: \"status != nil and status != ''\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenXMLUsesSQLInclude_shouldExpandFragment(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <sql id="BaseColumns">id, name</sql>
  <select id="FindByID">
    select <include refid="BaseColumns"/>
    from sys_user
    where id = #{id}
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	if strings.Contains(output, "DynamicSQLNodeInclude") {
		t.Fatalf("include node should be expanded in generated metadata:\n%s", output)
	}
	if !strings.Contains(output, `Text: "BaseColumns"`) && !strings.Contains(output, `Text: "id, name"`) {
		t.Fatalf("expected include fragment text in generated metadata:\n%s", output)
	}
}

func TestGenerate_whenSQLParameterMissing_shouldReject(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id from sys_user where id = #{missing}")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "SQL parameter \"missing\"") {
		t.Fatalf("expected missing parameter error, got %v", err)
	}
}

func writeSamplePackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

type Profile struct{}

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`json:"id" goark-orm:"column='id';primary-key=true;auto-increment=true"`+"`"+`
	Name string `+"`"+`json:"name" goark-orm:"column='name';size=64;nullable=false"`+"`"+`
	Status string `+"`"+`json:"status" goark-orm:"column='status'"`+"`"+`
	Profile Profile `+"`"+`json:"profile" goark-orm:"column='profile';type='jsonb';type-handler='json'"`+"`"+`
	Temp string `+"`"+`json:"-" goark-orm:"transient=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListByStatus(ctx context.Context, status string) ([]User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <resultMap id="UserResult" type="User">
    <id property="ID" column="id"/>
    <result property="Name" column="name"/>
    <result property="Status" column="status"/>
    <result property="Profile" column="profile" typeHandler="json"/>
  </resultMap>
  <select id="FindByID" resultMap="UserResult">
    select id, name, status, profile
    from sys_user
    where id = #{id}
  </select>
</mapper>
`)
	return dir
}

func mustWriteFile(t *testing.T, path string, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
}
