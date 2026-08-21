package ormgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"goark.dev/orm"
)

var sqlParameterPattern = regexp.MustCompile(`#\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}`)

// ScanPackage 扫描单个 Go package 的 ORM 元数据。
func ScanPackage(spec GenerateSpec) (*PackageModel, error) {
	spec.Dir = strings.TrimSpace(spec.Dir)
	if spec.Dir == "" {
		spec.Dir = "."
	}
	fset := token.NewFileSet()
	packages, err := parser.ParseDir(fset, spec.Dir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, "_test.go") &&
			!strings.HasPrefix(name, "zz_goark_orm_")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if len(packages) == 0 {
		return nil, fmt.Errorf("goark-orm: no Go package found in %s", spec.Dir)
	}
	packageNames := make([]string, 0, len(packages))
	for name := range packages {
		packageNames = append(packageNames, name)
	}
	sort.Strings(packageNames)
	packageName := strings.TrimSpace(spec.PackageName)
	if packageName == "" {
		if len(packageNames) != 1 {
			return nil, fmt.Errorf("goark-orm: multiple Go packages found in %s: %s", spec.Dir, strings.Join(packageNames, ", "))
		}
		packageName = packageNames[0]
	}
	pkg := packages[packageName]
	if pkg == nil {
		return nil, fmt.Errorf("goark-orm: package %q not found in %s", packageName, spec.Dir)
	}

	model := &PackageModel{
		Dir:         spec.Dir,
		PackageName: packageName,
	}
	files := sortedFiles(fset, pkg)
	for _, file := range files {
		if err := scanFile(model, fset, file, spec); err != nil {
			return nil, err
		}
	}
	if err := finalizeModel(model, spec); err != nil {
		return nil, err
	}
	return model, nil
}

func sortedFiles(fset *token.FileSet, pkg *ast.Package) []*ast.File {
	files := make([]*ast.File, 0, len(pkg.Files))
	for _, file := range pkg.Files {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool {
		return fset.Position(files[i].Package).Filename < fset.Position(files[j].Package).Filename
	})
	return files
}

func scanFile(model *PackageModel, fset *token.FileSet, file *ast.File, spec GenerateSpec) error {
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		declAnnotations, err := parseAnnotations(genDecl.Doc)
		if err != nil {
			return err
		}
		for _, rawSpec := range genDecl.Specs {
			typeSpec, ok := rawSpec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			specAnnotations, err := parseAnnotations(typeSpec.Doc)
			if err != nil {
				return err
			}
			annotations := mergeAnnotations(declAnnotations, specAnnotations)
			if _, ok := findAnnotation(annotations, "entity"); ok {
				entity, err := buildEntity(model, fset, typeSpec, annotations, spec)
				if err != nil {
					return err
				}
				model.Entities = append(model.Entities, entity)
			}
			if _, ok := findAnnotation(annotations, "mapper"); ok {
				mapper, err := buildMapper(model, fset, typeSpec, annotations)
				if err != nil {
					return err
				}
				model.Mappers = append(model.Mappers, mapper)
			}
		}
	}
	return nil
}

func buildEntity(model *PackageModel, fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []annotation, spec GenerateSpec) (EntityModel, error) {
	entityAnnotation, _ := findAnnotation(annotations, "entity")
	table := strings.TrimSpace(entityAnnotation.Args["table"])
	if table == "" {
		return EntityModel{}, fmt.Errorf("goark-orm: entity %s missing required table", typeSpec.Name.Name)
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return EntityModel{}, fmt.Errorf("goark-orm: entity %s requires struct type", typeSpec.Name.Name)
	}
	entity := EntityModel{
		TypeName: typeSpec.Name.Name,
		Table:    table,
	}
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		if field.Tag == nil {
			for _, name := range field.Names {
				return EntityModel{}, fmt.Errorf("goark-orm: field %s.%s missing goark-orm tag", entity.TypeName, name.Name)
			}
		}
		tag, ok, err := parseGoarkORMStructTag(field.Tag.Value)
		if err != nil {
			return EntityModel{}, err
		}
		if !ok {
			for _, name := range field.Names {
				return EntityModel{}, fmt.Errorf("goark-orm: field %s.%s missing goark-orm tag", entity.TypeName, name.Name)
			}
		}
		for _, name := range field.Names {
			column, err := buildColumn(entity.TypeName, name.Name, exprString(fset, field.Type), tag)
			if err != nil {
				return EntityModel{}, err
			}
			if !column.Transient {
				entity.Columns = append(entity.Columns, column)
			}
		}
	}
	if err := validateEntity(entity, spec); err != nil {
		return EntityModel{}, err
	}
	return entity, nil
}

func buildColumn(entityName string, fieldName string, fieldType string, tag fieldTag) (ColumnModel, error) {
	column := ColumnModel{FieldName: fieldName, FieldType: fieldType}
	if transient, _, err := tagBool(tag, "transient"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if transient {
		column.Transient = true
		return column, nil
	}
	if value, ok, err := tagString(tag, "column"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.ColumnName = value
	}
	if column.ColumnName == "" {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s missing column", entityName, fieldName)
	}
	if value, _, err := tagBool(tag, "primary-key"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else {
		column.PrimaryKey = value
	}
	if value, _, err := tagBool(tag, "auto-increment"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else {
		column.AutoIncrement = value
	}
	if value, ok, err := tagBool(tag, "nullable"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.Nullable = &value
	}
	if value, ok, err := tagInt(tag, "size"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.Size = &value
	}
	stringAttrs := map[string]*string{
		"type":         &column.DBType,
		"default":      &column.DefaultValue,
		"type-handler": &column.TypeHandler,
	}
	for key, target := range stringAttrs {
		if value, ok, err := tagString(tag, key); err != nil {
			return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
		} else if ok {
			*target = value
		}
	}
	boolAttrs := map[string]*bool{
		"version":     &column.Version,
		"soft-delete": &column.SoftDelete,
		"created-at":  &column.CreatedAt,
		"updated-at":  &column.UpdatedAt,
	}
	for key, target := range boolAttrs {
		if value, _, err := tagBool(tag, key); err != nil {
			return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
		} else {
			*target = value
		}
	}
	return column, nil
}

func validateEntity(entity EntityModel, spec GenerateSpec) error {
	primaryKeys := 0
	versionFields := 0
	softDeleteFields := 0
	createdAtFields := 0
	updatedAtFields := 0
	typeHandlers := typeHandlerSet(spec)
	for _, column := range entity.Columns {
		if column.PrimaryKey {
			primaryKeys++
		}
		if column.AutoIncrement && !column.PrimaryKey {
			return fmt.Errorf("goark-orm: field %s.%s auto-increment requires primary-key=true", entity.TypeName, column.FieldName)
		}
		if column.Version {
			versionFields++
		}
		if column.SoftDelete {
			softDeleteFields++
		}
		if column.CreatedAt {
			createdAtFields++
		}
		if column.UpdatedAt {
			updatedAtFields++
		}
		if column.TypeHandler != "" {
			if _, ok := typeHandlers[column.TypeHandler]; !ok {
				return fmt.Errorf("goark-orm: field %s.%s uses unregistered type-handler %q", entity.TypeName, column.FieldName, column.TypeHandler)
			}
		}
	}
	if primaryKeys == 0 {
		return fmt.Errorf("goark-orm: entity %s missing primary-key field", entity.TypeName)
	}
	if versionFields > 1 {
		return fmt.Errorf("goark-orm: entity %s has multiple version fields", entity.TypeName)
	}
	if softDeleteFields > 1 {
		return fmt.Errorf("goark-orm: entity %s has multiple soft-delete fields", entity.TypeName)
	}
	if createdAtFields > 1 {
		return fmt.Errorf("goark-orm: entity %s has multiple created-at fields", entity.TypeName)
	}
	if updatedAtFields > 1 {
		return fmt.Errorf("goark-orm: entity %s has multiple updated-at fields", entity.TypeName)
	}
	return nil
}

func typeHandlerSet(spec GenerateSpec) map[string]struct{} {
	out := map[string]struct{}{
		"json":    {},
		"time":    {},
		"decimal": {},
	}
	for _, item := range spec.TypeHandlers {
		item = strings.TrimSpace(item)
		if item != "" {
			out[item] = struct{}{}
		}
	}
	return out
}

func buildMapper(model *PackageModel, fset *token.FileSet, typeSpec *ast.TypeSpec, annotations []annotation) (MapperModel, error) {
	mapperAnnotation, _ := findAnnotation(annotations, "mapper")
	namespace := strings.TrimSpace(mapperAnnotation.Args["namespace"])
	if namespace == "" {
		return MapperModel{}, fmt.Errorf("goark-orm: mapper %s missing required namespace", typeSpec.Name.Name)
	}
	interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
	if !ok {
		return MapperModel{}, fmt.Errorf("goark-orm: mapper %s requires interface type", typeSpec.Name.Name)
	}
	mapper := MapperModel{
		TypeName:     typeSpec.Name.Name,
		Namespace:    namespace,
		XML:          strings.TrimSpace(mapperAnnotation.Args["xml"]),
		ImplTypeName: "goarkORM" + typeSpec.Name.Name,
	}
	statementByID := make(map[string]StatementModel)
	if mapper.XML != "" {
		xmlPath := filepath.Join(model.Dir, filepath.FromSlash(mapper.XML))
		xmlMapper, err := parseXMLMapper(xmlPath)
		if err != nil {
			return MapperModel{}, err
		}
		if xmlMapper.Namespace != namespace {
			return MapperModel{}, fmt.Errorf("goark-orm: XML namespace %q does not match mapper namespace %q", xmlMapper.Namespace, namespace)
		}
		resultMaps, err := xmlResultMaps(xmlMapper)
		if err != nil {
			return MapperModel{}, err
		}
		mapper.ResultMaps = resultMaps
		statements, err := xmlStatements(namespace, xmlMapper)
		if err != nil {
			return MapperModel{}, err
		}
		for _, statement := range statements {
			statementByID[statement.ID] = statement
		}
	}
	usedXMLStatements := make(map[string]struct{})
	for _, method := range interfaceType.Methods.List {
		methods, err := buildMapperMethods(fset, namespace, method, statementByID)
		if err != nil {
			return MapperModel{}, err
		}
		for _, method := range methods {
			if method.Statement.Source == orm.StatementSourceXML {
				usedXMLStatements[method.Name] = struct{}{}
			}
		}
		mapper.Methods = append(mapper.Methods, methods...)
	}
	for id := range statementByID {
		if _, ok := usedXMLStatements[id]; !ok {
			return MapperModel{}, fmt.Errorf("goark-orm: XML statement %s does not match any method on mapper %s", id, mapper.TypeName)
		}
	}
	sort.SliceStable(mapper.Methods, func(i, j int) bool {
		return mapper.Methods[i].Name < mapper.Methods[j].Name
	})
	for _, method := range mapper.Methods {
		mapper.Statements = append(mapper.Statements, method.Statement)
	}
	return mapper, nil
}

func buildMapperMethods(fset *token.FileSet, namespace string, field *ast.Field, xmlStatements map[string]StatementModel) ([]MethodModel, error) {
	if len(field.Names) == 0 {
		return nil, fmt.Errorf("goark-orm: embedded mapper interfaces are not supported in V1")
	}
	out := make([]MethodModel, 0, len(field.Names))
	for _, name := range field.Names {
		methodName := name.Name
		funcType, ok := field.Type.(*ast.FuncType)
		if !ok {
			return nil, fmt.Errorf("goark-orm: mapper method %s must be function", methodName)
		}
		method, err := buildMethodSignature(fset, methodName, funcType)
		if err != nil {
			return nil, err
		}
		annotations, err := parseAnnotations(field.Doc)
		if err != nil {
			return nil, err
		}
		annotationStatement, hasAnnotationStatement, err := statementFromMethodAnnotation(namespace, methodName, annotations)
		if err != nil {
			return nil, err
		}
		xmlStatement, hasXMLStatement := xmlStatements[methodName]
		if hasAnnotationStatement && hasXMLStatement {
			return nil, fmt.Errorf("goark-orm: method %s is declared by both XML and annotation", methodName)
		}
		switch {
		case hasAnnotationStatement:
			method.Statement = annotationStatement
		case hasXMLStatement:
			method.Statement = xmlStatement
		default:
			return nil, fmt.Errorf("goark-orm: mapper method %s has no statement", methodName)
		}
		applyMethodSignatureToStatement(&method)
		method.Command = method.Statement.Command
		out = append(out, method)
	}
	return out, nil
}

func applyMethodSignatureToStatement(method *MethodModel) {
	if method.Statement.ParameterType == "" && len(method.Params) == 2 {
		method.Statement.ParameterType = normalizeTypeName(method.Params[1].Type)
	}
	if method.Statement.ResultType == "" {
		method.Statement.ResultType = normalizeResultType(method.ResultType)
	}
}

func normalizeResultType(typ string) string {
	typ = strings.TrimSpace(typ)
	for strings.HasPrefix(typ, "[]") {
		typ = strings.TrimSpace(strings.TrimPrefix(typ, "[]"))
	}
	return normalizeTypeName(typ)
}

func buildMethodSignature(fset *token.FileSet, methodName string, fn *ast.FuncType) (MethodModel, error) {
	method := MethodModel{Name: methodName}
	if fn.Params == nil || len(fn.Params.List) == 0 {
		return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s first parameter must be context.Context", methodName)
	}
	paramIndex := 0
	for _, field := range fn.Params.List {
		names := field.Names
		if len(names) == 0 {
			return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s requires named parameters", methodName)
		}
		for _, name := range names {
			method.Params = append(method.Params, ParamModel{
				Name: name.Name,
				Type: exprString(fset, field.Type),
			})
			paramIndex++
		}
	}
	if len(method.Params) == 0 || method.Params[0].Type != "context.Context" {
		return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s first parameter must be context.Context", methodName)
	}
	if len(method.Params[0].Name) == 0 {
		return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s context parameter must be named", methodName)
	}
	if fn.Results == nil || len(fn.Results.List) != 2 {
		return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s must return (T, error)", methodName)
	}
	if exprString(fset, fn.Results.List[1].Type) != "error" {
		return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s last return value must be error", methodName)
	}
	method.ResultType = exprString(fset, fn.Results.List[0].Type)
	_ = paramIndex
	return method, nil
}

func statementFromMethodAnnotation(namespace string, methodName string, annotations []annotation) (StatementModel, bool, error) {
	var selected annotation
	command := orm.StatementCommand("")
	count := 0
	for _, name := range []string{"select", "insert", "update", "delete"} {
		item, ok := findAnnotation(annotations, name)
		if !ok {
			continue
		}
		selected = item
		command = orm.StatementCommand(name)
		count++
	}
	if count == 0 {
		return StatementModel{}, false, nil
	}
	if count > 1 {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s has multiple SQL annotations", methodName)
	}
	sql := strings.TrimSpace(selected.Args["sql"])
	if sql == "" {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s requires sql", methodName, selected.Name)
	}
	if strings.Contains(sql, "${") {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s uses forbidden ${}", methodName, selected.Name)
	}
	useGeneratedKeys := false
	if value := strings.TrimSpace(selected.Args["useGeneratedKeys"]); value != "" {
		switch value {
		case "true":
			useGeneratedKeys = true
		case "false":
			useGeneratedKeys = false
		default:
			return StatementModel{}, true, fmt.Errorf("goark-orm: method %s useGeneratedKeys requires boolean value", methodName)
		}
	}
	statement := StatementModel{
		ID:               methodName,
		Namespace:        namespace,
		FullName:         namespace + "." + methodName,
		Command:          command,
		Source:           orm.StatementSourceAnnotation,
		SQL:              sql,
		UseGeneratedKeys: useGeneratedKeys,
		KeyProperty:      strings.TrimSpace(selected.Args["keyProperty"]),
		Parameters:       statementParameters(sql),
	}
	return statement, true, nil
}

func validateMethodStatement(model *PackageModel, method MethodModel) error {
	switch method.Statement.Command {
	case orm.StatementCommandSelect:
		if method.ResultType == "int64" || method.ResultType == "int" || method.ResultType == "string" || method.ResultType == "bool" {
			return validateSQLParameters(model, method)
		}
		return validateSQLParameters(model, method)
	case orm.StatementCommandInsert, orm.StatementCommandUpdate, orm.StatementCommandDelete:
		if method.ResultType != "int64" {
			return fmt.Errorf("goark-orm: mapper method %s command %s must return (int64, error)", method.Name, method.Statement.Command)
		}
		return validateSQLParameters(model, method)
	default:
		return fmt.Errorf("goark-orm: mapper method %s has unsupported command %q", method.Name, method.Statement.Command)
	}
}

func resultMapExists(mapper MapperModel, id string) bool {
	for _, resultMap := range mapper.ResultMaps {
		if resultMap.ID == id {
			return true
		}
	}
	return false
}

func validateSQLParameters(model *PackageModel, method MethodModel) error {
	available := availableParameters(model, method)
	for _, name := range method.Statement.Parameters {
		if _, ok := available[name]; !ok {
			return fmt.Errorf("goark-orm: mapper method %s SQL parameter %q has no matching method parameter or entity field", method.Name, name)
		}
	}
	return nil
}

func availableParameters(model *PackageModel, method MethodModel) map[string]struct{} {
	out := make(map[string]struct{})
	dataParams := method.Params[1:]
	for _, param := range dataParams {
		out[param.Name] = struct{}{}
	}
	if len(dataParams) != 1 {
		return out
	}
	entityName := normalizeTypeName(dataParams[0].Type)
	for _, entity := range model.Entities {
		if entity.TypeName != entityName {
			continue
		}
		for _, column := range entity.Columns {
			out[column.FieldName] = struct{}{}
		}
	}
	return out
}

func normalizeTypeName(typ string) string {
	typ = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(typ), "*"))
	if index := strings.LastIndex(typ, "."); index >= 0 {
		return typ[index+1:]
	}
	return typ
}

func statementParameters(sql string) []string {
	matches := sqlParameterPattern.FindAllStringSubmatch(sql, -1)
	out := make([]string, 0, len(matches))
	seen := make(map[string]struct{})
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func finalizeModel(model *PackageModel, spec GenerateSpec) error {
	sort.SliceStable(model.Entities, func(i, j int) bool {
		return model.Entities[i].TypeName < model.Entities[j].TypeName
	})
	sort.SliceStable(model.Mappers, func(i, j int) bool {
		return model.Mappers[i].Namespace < model.Mappers[j].Namespace
	})
	namespaces := make(map[string]string)
	for _, mapper := range model.Mappers {
		if existing, ok := namespaces[mapper.Namespace]; ok {
			return fmt.Errorf("goark-orm: duplicate mapper namespace %q on %s and %s", mapper.Namespace, existing, mapper.TypeName)
		}
		namespaces[mapper.Namespace] = mapper.TypeName
		for _, method := range mapper.Methods {
			if method.Statement.ResultMap != "" && !resultMapExists(mapper, method.Statement.ResultMap) {
				return fmt.Errorf("goark-orm: method %s references missing resultMap %q", method.Name, method.Statement.ResultMap)
			}
			if err := validateMethodStatement(model, method); err != nil {
				return err
			}
		}
	}
	_ = spec
	return nil
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var builder bytes.Buffer
	_ = printer.Fprint(&builder, fset, expr)
	return builder.String()
}

func lowerCamel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	r, size := utf8.DecodeRuneInString(value)
	if r == utf8.RuneError {
		return value
	}
	return string(unicode.ToLower(r)) + value[size:]
}
