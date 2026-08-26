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
		"Version: true",
		"SoftDelete: true",
		"CreatedAt: true",
		"UpdatedAt: true",
		"Fill: orm.FieldFillInsertUpdate",
		"type goarkORMUserFields struct",
		"var UserFields = goarkORMUserFields",
		"ID:        orm.NewField[User](\"id\")",
		"type goarkORMUserTypedFields struct",
		"var UserTypedFields = goarkORMUserTypedFields",
		"ID:        orm.NewTypedField[User, int64](\"id\")",
		"func NewUserBaseMapper(session orm.StatementSession) (*orm.BaseMapper[User, int64], error)",
		"func NewUserService(session orm.StatementSession) (*orm.Service[User, int64], error)",
		"Namespace: \"system.user.UserMapper\"",
		"Source: orm.StatementSource(\"xml\")",
		"Source: orm.StatementSource(\"annotation\")",
		"type goarkORMUserMapper struct",
		"func NewUserMapper(session orm.Session) UserMapper",
		"func (m *goarkORMUserMapper) FindByID(ctx context.Context, id int64) (*User, error)",
		"m.session.QueryOne(ctx, \"system.user.UserMapper.FindByID\", orm.NamedArgs{\"_parameter\": id, \"id\": id, \"param1\": id}, &out)",
		"func (m *goarkORMUserMapper) ListByStatus(ctx context.Context, status string) ([]User, error)",
		"m.session.Query(ctx, \"system.user.UserMapper.ListByStatus\", orm.NamedArgs{\"_parameter\": status, \"param1\": status, \"status\": status}, &out)",
		"func (m *goarkORMUserMapper) ListPage(ctx context.Context, status string, page orm.PageRequest) (orm.Page[User], error)",
		"return orm.QueryPage[User](ctx, m.session, \"system.user.UserMapper.ListPage\", orm.NamedArgs{\"_parameter\": status, \"param1\": status, \"status\": status}, page)",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenSQLUsesNestedParameterPath_shouldRenderMyBatisAliases(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	Status string `+"`"+`goark-orm:"column='status'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByUser(ctx context.Context, user *User) (*User, error)
	ListByIDs(ctx context.Context, ids []int64) ([]User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="FindByUser" resultType="User">
    select id, name, status from sys_user
    where id = #{user.ID} and name = #{param1.name} and status = #{_parameter.Status}
  </select>
  <select id="ListByIDs" resultType="User">
    select id, name, status from sys_user where id in
    <foreach collection="list" item="id" open="(" separator="," close=")">
      #{id}
    </foreach>
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`"_parameter": user`,
		`"param1": user`,
		`"user": user`,
		`"id": user.ID`,
		`"name": user.Name`,
		`"array": ids`,
		`"collection": ids`,
		`"list": ids`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenXMLCallUsesOutAndResultSets_shouldRenderCallableMapper(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:entity(table="sys_role")
type Role struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Code string `+"`"+`goark-orm:"column='code'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	CallReport(ctx context.Context, status string, total *int64, users *[]User, roles *[]Role) error
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <call id="CallReport" statementType="CALLABLE">
    call load_user_report(#{status}, #{total})
    <parameter property="status" mode="IN"/>
    <parameter property="total" mode="OUT" jdbcType="BIGINT"/>
    <resultSet name="users" resultType="User"/>
    <resultSet name="roles" resultType="Role"/>
  </call>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`registry.RegisterRowScanner("User", orm.RowScannerFunc(goarkORMScanUserRow))`,
		`registry.RegisterRowScanner("Role", orm.RowScannerFunc(goarkORMScanRoleRow))`,
		`func goarkORMScanUserRow(ctx context.Context, columns []string, row orm.RowScannerRow, dest any) error`,
		`user, ok := dest.(*User)`,
		`targets[index] = &user.ID`,
		`Command: orm.StatementCommand("call")`,
		`StatementType: orm.StatementTypeCallable`,
		`ParameterModes: []orm.ParameterMeta{{Name: "status"}, {Name: "total", Mode: orm.ParameterModeOut, JDBCType: "BIGINT"}}`,
		`ResultSets: []orm.ResultSetMeta{{Name: "users", ResultType: "User"}, {Name: "roles", ResultType: "Role"}}`,
		`func (m *goarkORMUserMapper) CallReport(ctx context.Context, status string, total *int64, users *[]User, roles *[]Role) error`,
		`_, err := orm.Call(ctx, m.session, "system.user.UserMapper.CallReport", orm.NamedArgs{"param1": status, "param2": total, "status": status, "total": total}, users, roles)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenAnnotationCallUsesOutAndCallResult_shouldRenderCallResult(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import (
	"context"

	orm "goark.dev/orm"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:call(sql="call count_users(#{status}, #{total})", parameters="status:IN,total:OUT:BIGINT")
	CountByStatus(ctx context.Context, status string, total *int64) (orm.CallResult, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`StatementType: orm.StatementTypeCallable`,
		`ParameterModes: []orm.ParameterMeta{{Name: "status"}, {Name: "total", Mode: orm.ParameterModeOut, JDBCType: "BIGINT"}}`,
		`func (m *goarkORMUserMapper) CountByStatus(ctx context.Context, status string, total *int64) (orm.CallResult, error)`,
		`return orm.Call(ctx, m.session, "system.user.UserMapper.CountByStatus", orm.NamedArgs{"param1": status, "param2": total, "status": status, "total": total})`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenStatementOptionsDeclared_shouldRenderStatementOptions(t *testing.T) {
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
	//goark-orm:select(sql="select id, name from sys_user where id = #{id}", timeout="2s", fetchSize=128, resultSetType="FORWARD_ONLY", resultOrdered=true)
	FindByID(ctx context.Context, id int64) (*User, error)
	InsertUser(ctx context.Context, user *User) (int64, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <insert id="InsertUser" useGeneratedKeys="true" keyProperty="ID" keyColumn="id" timeout="3" fetchSize="64">
    insert into sys_user(name) values(#{Name})
  </insert>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`Options: orm.StatementOptions{Timeout: 2000000000, FetchSize: 128, ResultSetType: orm.ResultSetTypeForwardOnly, ResultOrdered: true}`,
		`Options: orm.StatementOptions{Timeout: 3000000000, FetchSize: 64, KeyColumn: "id"}`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenAnnotationUsesRawSQLToken_shouldKeepRawPlaceholder(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import (
	"context"

	orm "goark.dev/orm"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id, name from ${table} where id = #{id}")
	FindFrom(ctx context.Context, table orm.RawIdentifier, id int64) (*User, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`${table}`,
		`"table": table`,
		`"id": id`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenMapperUsesStreamingSignatures_shouldRenderCursorAndEachCalls(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import (
	"context"

	orm "goark.dev/orm"
)

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	Status string `+"`"+`goark-orm:"column='status'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListCursor(ctx context.Context, status string) (*orm.Cursor[User], error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListEach(ctx context.Context, status string, handler orm.ResultHandler[User]) error
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`ResultType: "User"`,
		`func (m *goarkORMUserMapper) ListCursor(ctx context.Context, status string) (*orm.Cursor[User], error)`,
		`return orm.QueryCursor[User](ctx, m.session, "system.user.UserMapper.ListCursor", orm.NamedArgs{"_parameter": status, "param1": status, "status": status})`,
		`func (m *goarkORMUserMapper) ListEach(ctx context.Context, status string, handler orm.ResultHandler[User]) error`,
		`return orm.QueryEach[User](ctx, m.session, "system.user.UserMapper.ListEach", orm.NamedArgs{"_parameter": status, "param1": status, "status": status}, handler)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
	if strings.Contains(output, `"handler": handler`) {
		t.Fatalf("stream handler must not be bound as SQL argument:\n%s", output)
	}
}

func TestGenerate_whenMapperEmbedsLocalInterfaces_shouldFlattenMethods(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

type UserQueryMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)

	//goark-orm:select(sql="select id, name from sys_user where name = #{name}")
	FindByName(ctx context.Context, name string) (*User, error)
}

type UserWriteMapper interface {
	//goark-orm:update(sql="update sys_user set name = #{name} where id = #{id}")
	UpdateName(ctx context.Context, id int64, name string) (int64, error)
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	UserQueryMapper
	UserWriteMapper
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="FindByID" resultType="User">
    select id, name from sys_user where id = #{id}
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		`func (m *goarkORMUserMapper) FindByID(ctx context.Context, id int64) (*User, error)`,
		`m.session.QueryOne(ctx, "system.user.UserMapper.FindByID", orm.NamedArgs{"_parameter": id, "id": id, "param1": id}, &out)`,
		`func (m *goarkORMUserMapper) FindByName(ctx context.Context, name string) (*User, error)`,
		`m.session.QueryOne(ctx, "system.user.UserMapper.FindByName", orm.NamedArgs{"_parameter": name, "name": name, "param1": name}, &out)`,
		`func (m *goarkORMUserMapper) UpdateName(ctx context.Context, id int64, name string) (int64, error)`,
		`result, err := m.session.Exec(ctx, "system.user.UserMapper.UpdateName", orm.NamedArgs{"id": id, "name": name, "param1": id, "param2": name})`,
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
	mustWriteFile(t, filepath.Join(dir, "generated_api_test.go"), `package sample

import (
	"context"
	"testing"

	orm "goark.dev/orm"
)

func TestGeneratedBaseMapperAPICompiles(t *testing.T) {
	_ = orm.NewQueryWrapper[User]().Eq(UserFields.ID, int64(1)).OrderByAsc(UserFields.Name)
	_ = orm.SetTypedValue(orm.NewUpdateWrapper[User](), UserTypedFields.Name, "Alice")
	_ = orm.EqTypedValue(orm.NewUpdateWrapper[User](), UserTypedFields.ID, int64(1))
	var factory func(orm.StatementSession) (*orm.BaseMapper[User, int64], error) = NewUserBaseMapper
	if factory == nil {
		t.Fatal("base mapper factory is nil")
	}
	var serviceFactory func(orm.StatementSession) (*orm.Service[User, int64], error) = NewUserService
	if serviceFactory == nil {
		t.Fatal("service factory is nil")
	}
}

func TestGeneratedMapperCanUseTransactionSession(t *testing.T) {
	var factory *orm.SQLSessionFactory
	_ = func(ctx context.Context) error {
		return factory.InTx(ctx, nil, func(ctx context.Context, session orm.Session) error {
			mapper := NewUserMapper(session)
			_, err := mapper.ListByStatus(ctx, "ACTIVE")
			return err
		})
	}
}
`)
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

func TestGenerate_whenAnnotationContainsScript_shouldRenderDynamicSQLMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Status string `+"`"+`goark-orm:"column='status'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="<script>select id, status from sys_user <where><if test=\"status != nil and status != ''\">status = #{status}</if></where></script>")
	List(ctx context.Context, status string) ([]User, error)
}
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
		`Parameters: []string{"status"}`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenAnnotationUsesProvider_shouldRenderProviderMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Status string `+"`"+`goark-orm:"column='status'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(provider="UserSQL.ListByStatus")
	List(ctx context.Context, status string) ([]User, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	if !strings.Contains(output, `Provider: "UserSQL.ListByStatus"`) {
		t.Fatalf("generated source missing provider metadata:\n%s", output)
	}
}

func TestGenerate_whenAnnotationDeclaresSQLAndProvider_shouldReject(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id from sys_user", provider="UserSQL.List")
	List(ctx context.Context) ([]User, error)
}
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "declares both sql and provider") {
		t.Fatalf("expected sql/provider conflict, got %v", err)
	}
}

func TestGenerate_whenXMLUsesBind_shouldRenderBindMetadata(t *testing.T) {
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
	ListByName(ctx context.Context, name string) ([]User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="ListByName">
    <bind name="pattern" value="'%' + name + '%'"/>
    select id, name from sys_user where name like #{pattern}
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		"Kind: orm.DynamicSQLNodeBind",
		"Name: \"pattern\"",
		"Value: \"'%' + name + '%'\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenXMLHasDatabaseID_shouldPreferMatchingStatement(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="FindByID">
    select id from sys_user where id = #{id}
  </select>
  <select id="FindByID" databaseId="postgres">
    select id from sys_user where id = #{id} for update
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir, DatabaseID: "postgres"})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	if !strings.Contains(output, `DatabaseID: "postgres"`) || !strings.Contains(output, `for update`) {
		t.Fatalf("expected postgres statement to be selected:\n%s", output)
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

func TestGenerate_whenXMLDeclaresCache_shouldRenderCacheMetadataAndStatementPolicies(t *testing.T) {
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
	Touch(ctx context.Context, id int64) (int64, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <cache eviction="LRU" size="32" flushInterval="60000" readOnly="true" blocking="true"/>
  <select id="FindByID" useCache="false" flushCache="true">
    select id, name from sys_user where id = #{id}
  </select>
  <update id="Touch" flushCache="false">
    update sys_user set name = name where id = #{id}
  </update>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	expected := []string{
		`Cache: orm.CacheMeta{Enabled: true, Eviction: "LRU", Size: 32, FlushIntervalMillis: 60000, ReadOnly: true, Blocking: true}`,
		`UseCache: orm.StatementCacheDisabled`,
		`FlushCache: orm.StatementCacheEnabled`,
		`FlushCache: orm.StatementCacheDisabled`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, string(source))
		}
	}
}

func TestGenerate_whenXMLDeclaresCacheRef_shouldRenderCacheRefMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_profile")
type Profile struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	UserID int64 `+"`"+`goark-orm:"column='user_id'"`+"`"+`
}

//goark-orm:mapper(namespace="system.profile.ProfileMapper", xml="mapper/profile_mapper.xml")
type ProfileMapper interface {
	FindByUserID(ctx context.Context, userID int64) (*Profile, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "profile_mapper.xml"), `<mapper namespace="system.profile.ProfileMapper">
  <cache-ref namespace="system.user.UserMapper"/>
  <select id="FindByUserID">
    select id, user_id from sys_profile where user_id = #{userID}
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	expected := `Cache: orm.CacheMeta{Enabled: true, RefNamespace: "system.user.UserMapper"}`
	if !strings.Contains(output, expected) {
		t.Fatalf("generated source missing cache-ref metadata:\n%s", source)
	}
}

func TestGenerate_whenXMLInsertDeclaresSelectKey_shouldRenderSelectKeyMetadata(t *testing.T) {
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
	Insert(ctx context.Context, user *User) (int64, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <insert id="Insert" parameterType="User" keyProperty="ID">
    <selectKey keyProperty="ID" resultType="int64" order="BEFORE">
      select nextval('sys_user_id_seq')
    </selectKey>
    insert into sys_user(id, name) values(#{ID}, #{Name})
  </insert>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	expected := []string{
		`KeyProperty: "ID"`,
		`SelectKey: orm.SelectKeyMeta{Enabled: true, KeyProperty: "ID", ResultType: "int64", Order: orm.SelectKeyOrderBefore, SQL: "select nextval('sys_user_id_seq')"}`,
		`return result.LastInsertID, nil`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, source)
		}
	}
}

func TestGenerate_whenEntityDeclaresIDType_shouldRenderIDTypeMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true;id-type='assign-id'"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:insert(sql="insert into sys_user(id, name) values(#{ID}, #{Name})")
	Insert(ctx context.Context, user *User) (int64, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	if !strings.Contains(output, "IDType: orm.IDTypeAssignID") {
		t.Fatalf("generated source missing id type metadata:\n%s", output)
	}
}

func TestGenerate_whenEntityDeclaresAdvancedFieldMetadata_shouldRenderColumnMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true;key-column='user_id_key'"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name';select=false;insert-strategy='not-empty';update-strategy='never';where-strategy='not-null';condition='%s like #{%s}'"`+"`"+`
	Amount string `+"`"+`goark-orm:"column='amount';type='decimal';size=18;numeric-scale=2"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id, name, amount from sys_user where id = #{id}")
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	expected := []string{
		`KeyColumn: "user_id_key"`,
		`SelectDisabled: true`,
		`InsertStrategy: orm.FieldStrategyNotEmpty`,
		`UpdateStrategy: orm.FieldStrategyNever`,
		`WhereStrategy: orm.FieldStrategyNotNull`,
		`Condition: "%s like #{%s}"`,
		`NumericScale: goarkORMIntPtr(2)`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, source)
		}
	}
}

func TestGenerate_whenAnnotationDeclaresInterceptorIgnore_shouldRenderStatementMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper")
type UserMapper interface {
	//goark-orm:select(sql="select id from sys_user", interceptorIgnore="tenant,blockAttack")
	List(ctx context.Context) ([]User, error)
}
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	if !strings.Contains(output, `InterceptorIgnores: []string{"blockAttack", "tenant"}`) {
		t.Fatalf("generated source missing interceptor ignores:\n%s", source)
	}
}

func TestGenerate_whenXMLDeclaresInterceptorIgnore_shouldRenderStatementMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	List(ctx context.Context) ([]User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="List" interceptorIgnore="pagination tenant">
    select id from sys_user
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	if !strings.Contains(output, `InterceptorIgnores: []string{"pagination", "tenant"}`) {
		t.Fatalf("generated source missing XML interceptor ignores:\n%s", source)
	}
}

func TestGenerate_whenXMLResultMapUsesAssociationAndCollection_shouldRenderNestedMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="orders")
type Order struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	User User `+"`"+`goark-orm:"transient=true"`+"`"+`
	Items []OrderItem `+"`"+`goark-orm:"transient=true"`+"`"+`
}

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:entity(table="order_item")
type OrderItem struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	SKU string `+"`"+`goark-orm:"column='sku'"`+"`"+`
}

//goark-orm:mapper(namespace="system.order.OrderMapper", xml="mapper/order_mapper.xml")
type OrderMapper interface {
	FindByID(ctx context.Context, id int64) (*Order, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "order_mapper.xml"), `<mapper namespace="system.order.OrderMapper">
  <resultMap id="OrderResult" type="Order">
    <id property="ID" column="order_id"/>
    <result property="Name" column="order_name"/>
    <association property="User" type="User">
      <id property="ID" column="user_id"/>
      <result property="Name" column="user_name"/>
    </association>
    <collection property="Items" ofType="OrderItem">
      <id property="ID" column="item_id"/>
      <result property="SKU" column="item_sku"/>
    </collection>
  </resultMap>
  <select id="FindByID" resultMap="OrderResult">
    select 1
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		"Associations: []orm.ResultAssociationMeta{",
		"Property: \"User\"",
		"Collections: []orm.ResultCollectionMeta{",
		"Property: \"Items\"",
		"TypeName: \"OrderItem\"",
		"Column: \"item_sku\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenXMLResultMapUsesNestedSelect_shouldRenderNestedSelectMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="orders")
type Order struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	UserID int64 `+"`"+`goark-orm:"column='user_id'"`+"`"+`
	User User `+"`"+`goark-orm:"transient=true"`+"`"+`
	Items []OrderItem `+"`"+`goark-orm:"transient=true"`+"`"+`
}

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:entity(table="order_item")
type OrderItem struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.order.OrderMapper", xml="mapper/order_mapper.xml")
type OrderMapper interface {
	FindByID(ctx context.Context, id int64) (*Order, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "order_mapper.xml"), `<mapper namespace="system.order.OrderMapper">
  <resultMap id="OrderResult" type="Order">
    <id property="ID" column="order_id"/>
    <association property="User" column="user_id" select="system.user.UserMapper.FindByID" fetchType="eager"/>
    <collection property="Items" column="order_id" select="system.order.OrderItemMapper.ListByOrderID" fetchType="lazy"/>
  </resultMap>
  <select id="FindByID" resultMap="OrderResult">
    select 1
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := string(source)
	expected := []string{
		"Select: \"system.user.UserMapper.FindByID\"",
		"Column: \"user_id\"",
		"FetchType: \"eager\"",
		"Select: \"system.order.OrderItemMapper.ListByOrderID\"",
		"FetchType: \"lazy\"",
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, output)
		}
	}
}

func TestGenerate_whenXMLResultMapUsesExtendsAutoMappingAndDiscriminator_shouldRenderAdvancedMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	Type string `+"`"+`goark-orm:"column='type'"`+"`"+`
}

//goark-orm:entity(table="sys_admin")
type AdminUser struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	Level int `+"`"+`goark-orm:"column='level'"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <resultMap id="BaseUserResult" type="User" autoMapping="true">
    <id property="ID" column="id"/>
    <result property="Name" column="name"/>
  </resultMap>
  <resultMap id="UserResult" type="User" extends="BaseUserResult" autoMapping="false">
    <result property="Type" column="type"/>
    <discriminator column="type" javaType="string">
      <case value="admin" resultType="AdminUser">
        <result property="Level" column="level"/>
      </case>
      <case value="normal" resultMap="BaseUserResult"/>
    </discriminator>
  </resultMap>
  <select id="FindByID" resultMap="UserResult">
    select id, name, type, level from sys_user where id = #{id}
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	expected := []string{
		`ID: "UserResult"`,
		`Extends: "BaseUserResult"`,
		`AutoMapping: goarkORMBoolPtr(false)`,
		`Property: "ID", Column: "id", ID: true`,
		`Property: "Name", Column: "name"`,
		`Property: "Type", Column: "type"`,
		`Discriminator: orm.ResultDiscriminatorMeta{Column: "type", TypeName: "string", Cases: []orm.ResultDiscriminatorCaseMeta{{Value: "admin", ResultType: "AdminUser"`,
		`Property: "Level", Column: "level"`,
		`{Value: "normal", ResultMap: "BaseUserResult"`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, source)
		}
	}
}

func TestGenerate_whenXMLResultMapUsesConstructorPrefixAndNotNullColumn_shouldRenderDeepResultMapMetadata(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="invoice")
type Invoice struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
	User User `+"`"+`goark-orm:"transient=true"`+"`"+`
	Items []InvoiceItem `+"`"+`goark-orm:"transient=true"`+"`"+`
}

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	Name string `+"`"+`goark-orm:"column='name'"`+"`"+`
}

//goark-orm:entity(table="invoice_item")
type InvoiceItem struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
	SKU string `+"`"+`goark-orm:"column='sku'"`+"`"+`
}

//goark-orm:mapper(namespace="system.invoice.InvoiceMapper", xml="mapper/invoice_mapper.xml")
type InvoiceMapper interface {
	List(ctx context.Context) ([]Invoice, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "invoice_mapper.xml"), `<mapper namespace="system.invoice.InvoiceMapper">
  <resultMap id="InvoiceResult" type="Invoice" autoMapping="false">
    <constructor>
      <idArg name="ID" column="invoice_id"/>
      <arg name="Name" column="invoice_name"/>
    </constructor>
    <association property="User" type="User" columnPrefix="user_" notNullColumn="id">
      <id property="ID" column="id"/>
      <result property="Name" column="name"/>
    </association>
    <collection property="Items" ofType="InvoiceItem" columnPrefix="item_" notNullColumn="id, sku">
      <id property="ID" column="id"/>
      <result property="SKU" column="sku"/>
    </collection>
  </resultMap>
  <select id="List" resultMap="InvoiceResult">
    select 1
  </select>
</mapper>
`)

	source, err := Generate(GenerateSpec{Dir: dir})
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	output := strings.Join(strings.Fields(string(source)), " ")
	expected := []string{
		`AutoMapping: goarkORMBoolPtr(false)`,
		`Constructor: orm.ResultConstructorMeta{Args: []orm.ResultArgMeta{{Name: "ID", Column: "invoice_id", ID: true}, {Name: "Name", Column: "invoice_name"}}}`,
		`ColumnPrefix: "user_"`,
		`NotNullColumns: []string{"id"}`,
		`ColumnPrefix: "item_"`,
		`NotNullColumns: []string{"id", "sku"}`,
	}
	for _, fragment := range expected {
		if !strings.Contains(output, fragment) {
			t.Fatalf("generated source missing %q:\n%s", fragment, source)
		}
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

func TestGenerate_whenXMLResultMapAndResultTypeBothSet_shouldReject(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <resultMap id="UserResult" type="User">
    <id property="ID" column="id"/>
  </resultMap>
  <select id="FindByID" resultMap="UserResult" resultType="User">
    select id from sys_user where id = #{id}
  </select>
</mapper>
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "declares both resultMap and resultType") {
		t.Fatalf("expected resultMap/resultType error, got %v", err)
	}
}

func TestGenerate_whenXMLResultTypeUnknown_shouldReject(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="FindByID" resultType="Missing">
    select id from sys_user where id = #{id}
  </select>
</mapper>
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "unknown resultType") {
		t.Fatalf("expected resultType error, got %v", err)
	}
}

func TestGenerate_whenXMLParameterTypeDoesNotMatchMethod_shouldReject(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import "context"

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`goark-orm:"column='id';primary-key=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)
}
`)
	mustWriteFile(t, filepath.Join(dir, "mapper", "user_mapper.xml"), `<mapper namespace="system.user.UserMapper">
  <select id="FindByID" parameterType="User">
    select id from sys_user where id = #{id}
  </select>
</mapper>
`)

	_, err := Generate(GenerateSpec{Dir: dir})
	if err == nil || !strings.Contains(err.Error(), "parameterType") {
		t.Fatalf("expected parameterType error, got %v", err)
	}
}

func writeSamplePackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "mapper.go"), `package sample

import (
	"context"
	"time"

	orm "goark.dev/orm"
)

type Profile struct{}

//goark-orm:entity(table="sys_user")
type User struct {
	ID int64 `+"`"+`json:"id" goark-orm:"column='id';primary-key=true;auto-increment=true"`+"`"+`
	Name string `+"`"+`json:"name" goark-orm:"column='name';size=64;nullable=false"`+"`"+`
	Status string `+"`"+`json:"status" goark-orm:"column='status'"`+"`"+`
	Profile Profile `+"`"+`json:"profile" goark-orm:"column='profile';type='jsonb';type-handler='json'"`+"`"+`
	Version int64 `+"`"+`json:"version" goark-orm:"column='version';version=true"`+"`"+`
	Deleted bool `+"`"+`json:"deleted" goark-orm:"column='deleted';soft-delete=true"`+"`"+`
	CreatedAt time.Time `+"`"+`json:"createdAt" goark-orm:"column='created_at';created-at=true"`+"`"+`
	UpdatedAt time.Time `+"`"+`json:"updatedAt" goark-orm:"column='updated_at';updated-at=true;fill='insert_update'"`+"`"+`
	Temp string `+"`"+`json:"-" goark-orm:"transient=true"`+"`"+`
}

//goark-orm:mapper(namespace="system.user.UserMapper", xml="mapper/user_mapper.xml")
type UserMapper interface {
	FindByID(ctx context.Context, id int64) (*User, error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListByStatus(ctx context.Context, status string) ([]User, error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListPage(ctx context.Context, status string, page orm.PageRequest) (orm.Page[User], error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListCursor(ctx context.Context, status string) (*orm.Cursor[User], error)

	//goark-orm:select(sql="select id, name, status from sys_user where status = #{status}")
	ListEach(ctx context.Context, status string, handler orm.ResultHandler[User]) error
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
