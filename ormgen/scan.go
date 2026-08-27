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
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"goark.dev/orm"
)

var sqlParameterPattern = regexp.MustCompile(`#\{\s*([^{}]+?)\s*\}`)

type scanContext struct {
	fset       *token.FileSet
	interfaces map[string]*ast.InterfaceType
}

// ScanPackage 扫描单个 Go package 的 ORM 元数据。
func ScanPackage(spec GenerateSpec) (*PackageModel, error) {
	spec.Dir = strings.TrimSpace(spec.Dir)
	if spec.Dir == "" {
		spec.Dir = "."
	}
	naming, err := normalizeNamingConfig(spec.Naming)
	if err != nil {
		return nil, fmt.Errorf("goark-orm: naming %w", err)
	}
	spec.Naming = naming
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
	ctx := newScanContext(fset, pkg)
	files := sortedFiles(fset, pkg)
	for _, file := range files {
		if err := scanFile(model, ctx, file, spec); err != nil {
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

func newScanContext(fset *token.FileSet, pkg *ast.Package) *scanContext {
	ctx := &scanContext{
		fset:       fset,
		interfaces: make(map[string]*ast.InterfaceType),
	}
	for _, file := range sortedFiles(fset, pkg) {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, rawSpec := range genDecl.Specs {
				typeSpec, ok := rawSpec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				interfaceType, ok := typeSpec.Type.(*ast.InterfaceType)
				if ok {
					ctx.interfaces[typeSpec.Name.Name] = interfaceType
				}
			}
		}
	}
	return ctx
}

func scanFile(model *PackageModel, ctx *scanContext, file *ast.File, spec GenerateSpec) error {
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
				entity, err := buildEntity(model, ctx.fset, typeSpec, annotations, spec)
				if err != nil {
					return err
				}
				model.Entities = append(model.Entities, entity)
			}
			if _, ok := findAnnotation(annotations, "mapper"); ok {
				mapper, err := buildMapper(model, ctx, typeSpec, annotations, spec)
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
		table = deriveTableName(typeSpec.Name.Name, spec.Naming)
	}
	if table == "" {
		return EntityModel{}, fmt.Errorf("goark-orm: entity %s missing required table", typeSpec.Name.Name)
	}
	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok {
		return EntityModel{}, fmt.Errorf("goark-orm: entity %s requires struct type", typeSpec.Name.Name)
	}
	entity := EntityModel{
		TypeName:    typeSpec.Name.Name,
		Table:       table,
		KeySequence: firstAnnotationValue(entityAnnotation.Args, "keySequence", "key-sequence"),
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
			column, err := buildColumn(entity.TypeName, name.Name, exprString(fset, field.Type), tag, spec.Naming)
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

func buildColumn(entityName string, fieldName string, fieldType string, tag fieldTag, naming NamingConfig) (ColumnModel, error) {
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
		column.ColumnName = deriveColumnName(fieldName, naming)
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
	if value, ok, err := tagString(tag, "id-type"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		idType, err := orm.ParseIDType(value)
		if err != nil {
			return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
		}
		column.IDType = idType
	}
	if value, ok, err := tagString(tag, "fill"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		fill, err := orm.ParseFieldFill(value)
		if err != nil {
			return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
		}
		column.Fill = fill
	}
	for key, target := range map[string]*orm.FieldStrategy{
		"insert-strategy": &column.InsertStrategy,
		"update-strategy": &column.UpdateStrategy,
		"where-strategy":  &column.WhereStrategy,
	} {
		if value, ok, err := tagString(tag, key); err != nil {
			return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
		} else if ok {
			strategy, err := orm.ParseFieldStrategy(value)
			if err != nil {
				return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
			}
			*target = strategy
		}
	}
	if value, ok, err := tagBool(tag, "nullable"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.Nullable = &value
	}
	if value, ok, err := tagString(tag, "update"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.UpdateExpression = value
	}
	if value, ok, err := tagString(tag, "update-expression"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		if column.UpdateExpression != "" {
			return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s declares both update and update-expression", entityName, fieldName)
		}
		column.UpdateExpression = value
	}
	if value, ok, err := tagBool(tag, "select"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.SelectDisabled = !value
	}
	if value, _, err := tagBool(tag, "order-by"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else {
		column.OrderBy = value
	}
	if value, _, err := tagBool(tag, "order-desc"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else {
		column.OrderDesc = value
	}
	if value, ok, err := tagInt(tag, "size"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.Size = &value
	}
	if value, ok, err := tagInt(tag, "numeric-scale"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.NumericScale = &value
	}
	if value, ok, err := tagInt(tag, "order-priority"); err != nil {
		return ColumnModel{}, fmt.Errorf("goark-orm: field %s.%s %w", entityName, fieldName, err)
	} else if ok {
		column.OrderPriority = value
	}
	stringAttrs := map[string]*string{
		"type":         &column.DBType,
		"default":      &column.DefaultValue,
		"type-handler": &column.TypeHandler,
		"key-column":   &column.KeyColumn,
		"condition":    &column.Condition,
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
		if column.IDType != orm.IDTypeNone && !column.PrimaryKey {
			return fmt.Errorf("goark-orm: field %s.%s id-type requires primary-key=true", entity.TypeName, column.FieldName)
		}
		if column.AutoIncrement && column.IDType != orm.IDTypeNone && column.IDType != orm.IDTypeAuto {
			return fmt.Errorf("goark-orm: field %s.%s auto-increment conflicts with id-type %s", entity.TypeName, column.FieldName, column.IDType)
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

func buildMapper(model *PackageModel, ctx *scanContext, typeSpec *ast.TypeSpec, annotations []annotation, spec GenerateSpec) (MapperModel, error) {
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
		mapper.Cache = xmlMapper.Cache
		resultMaps, err := xmlResultMaps(xmlMapper)
		if err != nil {
			return MapperModel{}, err
		}
		mapper.ResultMaps = resultMaps
		statements, err := xmlStatements(namespace, xmlMapper, spec.DatabaseID)
		if err != nil {
			return MapperModel{}, err
		}
		for _, statement := range statements {
			statementByID[statement.ID] = statement
		}
	}
	usedXMLStatements := make(map[string]struct{})
	seenMethods := make(map[string]struct{})
	for _, method := range interfaceType.Methods.List {
		methods, err := buildMapperMethods(ctx, namespace, method, statementByID, nil)
		if err != nil {
			return MapperModel{}, err
		}
		for _, method := range methods {
			if _, exists := seenMethods[method.Name]; exists {
				return MapperModel{}, fmt.Errorf("goark-orm: mapper %s has duplicate method %s", mapper.TypeName, method.Name)
			}
			seenMethods[method.Name] = struct{}{}
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

func buildMapperMethods(ctx *scanContext, namespace string, field *ast.Field, xmlStatements map[string]StatementModel, stack map[string]struct{}) ([]MethodModel, error) {
	if len(field.Names) == 0 {
		embeddedName, err := embeddedInterfaceName(ctx.fset, field.Type)
		if err != nil {
			return nil, err
		}
		interfaceType, ok := ctx.interfaces[embeddedName]
		if !ok {
			return nil, fmt.Errorf("goark-orm: embedded mapper interface %s is not found in current package", embeddedName)
		}
		if stack == nil {
			stack = make(map[string]struct{})
		}
		if _, exists := stack[embeddedName]; exists {
			return nil, fmt.Errorf("goark-orm: embedded mapper interface %s has cyclic embedding", embeddedName)
		}
		stack[embeddedName] = struct{}{}
		defer delete(stack, embeddedName)
		out := make([]MethodModel, 0, len(interfaceType.Methods.List))
		seen := make(map[string]struct{})
		for _, embeddedField := range interfaceType.Methods.List {
			methods, err := buildMapperMethods(ctx, namespace, embeddedField, xmlStatements, stack)
			if err != nil {
				return nil, err
			}
			for _, method := range methods {
				if _, exists := seen[method.Name]; exists {
					return nil, fmt.Errorf("goark-orm: embedded mapper interface %s has duplicate method %s", embeddedName, method.Name)
				}
				seen[method.Name] = struct{}{}
				out = append(out, method)
			}
		}
		return out, nil
	}
	out := make([]MethodModel, 0, len(field.Names))
	for _, name := range field.Names {
		methodName := name.Name
		funcType, ok := field.Type.(*ast.FuncType)
		if !ok {
			return nil, fmt.Errorf("goark-orm: mapper method %s must be function", methodName)
		}
		method, err := buildMethodSignature(ctx.fset, methodName, funcType)
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

func embeddedInterfaceName(fset *token.FileSet, expr ast.Expr) (string, error) {
	switch item := expr.(type) {
	case *ast.Ident:
		if item.Name == "" {
			return "", fmt.Errorf("goark-orm: embedded mapper interface is empty")
		}
		return item.Name, nil
	default:
		return "", fmt.Errorf("goark-orm: embedded mapper interface %s must be local named interface", exprString(fset, expr))
	}
}

func applyMethodSignatureToStatement(method *MethodModel) {
	dataParams := methodDataParams(*method)
	if method.Statement.ParameterType == "" && len(dataParams) == 1 && !isCollectionParameterType(dataParams[0].Type) {
		method.Statement.ParameterType = normalizeTypeName(dataParams[0].Type)
	}
	if method.Statement.ResultType == "" {
		if itemType, ok := pageResultTypeArg(method.ResultType); ok {
			method.Statement.ResultType = normalizeResultType(itemType)
		} else if itemType, ok := cursorResultTypeArg(method.ResultType); ok {
			method.Statement.ResultType = normalizeResultType(itemType)
		} else if _, itemType, ok := resultHandlerParam(*method); ok {
			method.Statement.ResultType = normalizeResultType(itemType)
		} else {
			method.Statement.ResultType = normalizeResultType(method.ResultType)
		}
	}
}

func normalizeResultType(typ string) string {
	typ = strings.TrimSpace(typ)
	if itemType, ok := pageResultTypeArg(typ); ok {
		typ = itemType
	} else if itemType, ok := cursorResultTypeArg(typ); ok {
		typ = itemType
	}
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
	resultCount := 0
	if fn.Results != nil {
		resultCount = len(fn.Results.List)
	}
	switch resultCount {
	case 1:
		if exprString(fset, fn.Results.List[0].Type) != "error" {
			return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s single return value must be error", methodName)
		}
	case 2:
		if exprString(fset, fn.Results.List[1].Type) != "error" {
			return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s last return value must be error", methodName)
		}
		method.ResultType = exprString(fset, fn.Results.List[0].Type)
	default:
		return MethodModel{}, fmt.Errorf("goark-orm: mapper method %s must return (T, error) or error", methodName)
	}
	_ = paramIndex
	return method, nil
}

func statementFromMethodAnnotation(namespace string, methodName string, annotations []annotation) (StatementModel, bool, error) {
	var selected annotation
	command := orm.StatementCommand("")
	count := 0
	for _, name := range []string{"select", "insert", "update", "delete", "call"} {
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
	rawSQL := strings.TrimSpace(selected.Args["sql"])
	provider := strings.TrimSpace(selected.Args["provider"])
	switch {
	case rawSQL == "" && provider == "":
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s requires sql or provider", methodName, selected.Name)
	case rawSQL != "" && provider != "":
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s declares both sql and provider", methodName, selected.Name)
	}
	sql, dynamicSQL, err := parseAnnotationSQL(rawSQL)
	if err != nil {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s parses script failed: %w", methodName, selected.Name, err)
	}
	parameters, err := parseAnnotationCallableParameters(selected.Args)
	if err != nil {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s parses parameters failed: %w", methodName, selected.Name, err)
	}
	resultSets, err := parseAnnotationResultSets(selected.Args["resultSets"])
	if err != nil {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s parses resultSets failed: %w", methodName, selected.Name, err)
	}
	options, err := parseAnnotationStatementOptions(selected.Args)
	if err != nil {
		return StatementModel{}, true, fmt.Errorf("goark-orm: method %s annotation %s parses options failed: %w", methodName, selected.Name, err)
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
		ID:                 methodName,
		Namespace:          namespace,
		FullName:           namespace + "." + methodName,
		Command:            command,
		StatementType:      annotationStatementType(selected.Args["statementType"], command),
		Source:             orm.StatementSourceAnnotation,
		SQL:                sql,
		Provider:           provider,
		UseGeneratedKeys:   useGeneratedKeys,
		KeyProperty:        strings.TrimSpace(selected.Args["keyProperty"]),
		Options:            options,
		Parameters:         statementParameters(sql),
		ParameterModes:     parameters,
		ResultSets:         resultSets,
		DynamicSQL:         dynamicSQL,
		InterceptorIgnores: parseInterceptorIgnores(selected.Args["interceptorIgnore"]),
	}
	statement.Parameters = append(statement.Parameters, dynamicStatementParameters(statement.DynamicSQL)...)
	statement.Parameters = append(statement.Parameters, callableParameterNames(statement.ParameterModes)...)
	statement.Parameters = uniqueSorted(statement.Parameters)
	return statement, true, nil
}

func annotationStatementType(value string, command orm.StatementCommand) orm.StatementType {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case string(orm.StatementTypePrepared):
		return orm.StatementTypePrepared
	case string(orm.StatementTypeCallable):
		return orm.StatementTypeCallable
	default:
		if command == orm.StatementCommandCall {
			return orm.StatementTypeCallable
		}
		return ""
	}
}

func parseAnnotationStatementOptions(args map[string]string) (orm.StatementOptions, error) {
	timeout, err := parseAnnotationStatementTimeout(args)
	if err != nil {
		return orm.StatementOptions{}, err
	}
	fetchSize, err := parseAnnotationInt(args, "fetchSize")
	if err != nil {
		return orm.StatementOptions{}, err
	}
	if fetchSize < 0 {
		return orm.StatementOptions{}, fmt.Errorf("fetchSize must be >= 0")
	}
	resultSetType, err := orm.ParseResultSetType(args["resultSetType"])
	if err != nil {
		return orm.StatementOptions{}, err
	}
	resultOrdered, err := parseAnnotationBool(args, "resultOrdered")
	if err != nil {
		return orm.StatementOptions{}, err
	}
	return orm.StatementOptions{
		Timeout:       timeout,
		FetchSize:     fetchSize,
		ResultSetType: resultSetType,
		ResultOrdered: resultOrdered,
		KeyColumn:     strings.TrimSpace(args["keyColumn"]),
	}, nil
}

func parseAnnotationStatementTimeout(args map[string]string) (time.Duration, error) {
	value := strings.TrimSpace(firstNonEmpty(args["timeoutDuration"], args["timeout"]))
	if value == "" {
		return 0, nil
	}
	if duration, err := time.ParseDuration(value); err == nil {
		if duration < 0 {
			return 0, fmt.Errorf("timeout must be >= 0")
		}
		return duration, nil
	}
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("timeout requires duration or integer seconds")
	}
	if seconds < 0 {
		return 0, fmt.Errorf("timeout must be >= 0")
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseAnnotationInt(args map[string]string, name string) (int, error) {
	value := strings.TrimSpace(args[name])
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s requires integer value", name)
	}
	return parsed, nil
}

func parseAnnotationBool(args map[string]string, name string) (bool, error) {
	value := strings.TrimSpace(args[name])
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s requires boolean value", name)
	}
	return parsed, nil
}

func parseAnnotationCallableParameters(args map[string]string) ([]orm.ParameterMeta, error) {
	parameters := make([]orm.ParameterMeta, 0)
	if raw := strings.TrimSpace(args["parameters"]); raw != "" {
		parsed, err := parseCallableParameterList(raw)
		if err != nil {
			return nil, err
		}
		parameters = append(parameters, parsed...)
	}
	for _, item := range []struct {
		key  string
		mode orm.ParameterMode
	}{
		{key: "out", mode: orm.ParameterModeOut},
		{key: "inout", mode: orm.ParameterModeInOut},
	} {
		for _, name := range splitAnnotationList(args[item.key]) {
			parameters = upsertCallableParameter(parameters, orm.ParameterMeta{Name: name, Mode: item.mode})
		}
	}
	return parameters, nil
}

func parseCallableParameterList(raw string) ([]orm.ParameterMeta, error) {
	parts := splitAnnotationList(raw)
	parameters := make([]orm.ParameterMeta, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, ":")
		for index := range fields {
			fields[index] = strings.TrimSpace(fields[index])
		}
		if fields[0] == "" {
			return nil, fmt.Errorf("parameter name is required")
		}
		mode := orm.ParameterModeIn
		if len(fields) > 1 && fields[1] != "" {
			parsed, err := orm.ParseParameterMode(fields[1])
			if err != nil {
				return nil, err
			}
			mode = parsed
		}
		parameter := orm.ParameterMeta{Name: fields[0], Mode: mode}
		if len(fields) > 2 {
			parameter.JDBCType = fields[2]
		}
		if len(fields) > 3 {
			parameter.TypeHandler = fields[3]
		}
		if len(fields) > 4 {
			return nil, fmt.Errorf("parameter %q has too many fields", fields[0])
		}
		parameters = upsertCallableParameter(parameters, parameter)
	}
	return parameters, nil
}

func upsertCallableParameter(parameters []orm.ParameterMeta, parameter orm.ParameterMeta) []orm.ParameterMeta {
	parameter.Name = strings.TrimSpace(parameter.Name)
	if parameter.Name == "" {
		return parameters
	}
	for index := range parameters {
		if parameters[index].Name != parameter.Name {
			continue
		}
		if parameter.Mode != "" {
			parameters[index].Mode = parameter.Mode
		}
		if parameter.JDBCType != "" {
			parameters[index].JDBCType = parameter.JDBCType
		}
		if parameter.TypeHandler != "" {
			parameters[index].TypeHandler = parameter.TypeHandler
		}
		return parameters
	}
	return append(parameters, parameter)
}

func parseAnnotationResultSets(raw string) ([]orm.ResultSetMeta, error) {
	parts := splitAnnotationList(raw)
	resultSets := make([]orm.ResultSetMeta, 0, len(parts))
	for _, part := range parts {
		fields := strings.Split(part, ":")
		for index := range fields {
			fields[index] = strings.TrimSpace(fields[index])
		}
		if fields[0] == "" {
			return nil, fmt.Errorf("result set name is required")
		}
		resultSet := orm.ResultSetMeta{Name: fields[0]}
		if len(fields) > 1 {
			resultSet.ResultType = fields[1]
		}
		if len(fields) > 2 {
			resultSet.ResultMap = fields[2]
		}
		if len(fields) > 3 {
			return nil, fmt.Errorf("result set %q has too many fields", fields[0])
		}
		resultSets = append(resultSets, resultSet)
	}
	return resultSets, nil
}

func splitAnnotationList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	return trimmedStrings(parts)
}

func validateMethodStatement(model *PackageModel, method MethodModel) error {
	if err := validateStatementTypes(model, method); err != nil {
		return err
	}
	switch method.Statement.Command {
	case orm.StatementCommandSelect:
		if _, _, ok := resultHandlerParam(method); ok {
			if strings.TrimSpace(method.ResultType) != "" {
				return fmt.Errorf("goark-orm: mapper method %s with ResultHandler must return error", method.Name)
			}
			return validateSQLParameters(model, method)
		}
		if strings.TrimSpace(method.ResultType) == "" {
			return fmt.Errorf("goark-orm: mapper method %s select without ResultHandler must return (T, error)", method.Name)
		}
		if _, ok := cursorResultTypeArg(method.ResultType); ok {
			return validateSQLParameters(model, method)
		}
		if _, ok := pageResultTypeArg(method.ResultType); ok {
			if _, found := pageRequestParam(method); !found {
				return fmt.Errorf("goark-orm: mapper method %s returns page result but has no orm.PageRequest parameter", method.Name)
			}
			return validateSQLParameters(model, method)
		}
		if method.ResultType == "int64" || method.ResultType == "int" || method.ResultType == "string" || method.ResultType == "bool" {
			return validateSQLParameters(model, method)
		}
		return validateSQLParameters(model, method)
	case orm.StatementCommandInsert, orm.StatementCommandUpdate, orm.StatementCommandDelete:
		if method.ResultType != "int64" {
			return fmt.Errorf("goark-orm: mapper method %s command %s must return (int64, error)", method.Name, method.Statement.Command)
		}
		return validateSQLParameters(model, method)
	case orm.StatementCommandCall:
		if resultType := strings.TrimSpace(method.ResultType); resultType != "" && !isCallResultType(resultType) {
			return fmt.Errorf("goark-orm: mapper method %s call must return error or (orm.CallResult, error)", method.Name)
		}
		if err := validateCallableMethod(model, method); err != nil {
			return err
		}
		return validateSQLParameters(model, method)
	default:
		return fmt.Errorf("goark-orm: mapper method %s has unsupported command %q", method.Name, method.Statement.Command)
	}
}

func validateStatementTypes(model *PackageModel, method MethodModel) error {
	if err := validateParameterType(model, method); err != nil {
		return err
	}
	if method.Statement.Command == orm.StatementCommandSelect {
		if err := validateResultType(model, method); err != nil {
			return err
		}
	}
	if method.Statement.SelectKey.Enabled {
		if strings.TrimSpace(firstNonEmpty(method.Statement.SelectKey.KeyProperty, method.Statement.KeyProperty)) == "" {
			return fmt.Errorf("goark-orm: mapper method %s selectKey requires keyProperty", method.Name)
		}
		resultType := normalizeTypeName(method.Statement.SelectKey.ResultType)
		if resultType != "" && !isScalarType(resultType) && !entityExists(model, resultType) {
			return fmt.Errorf("goark-orm: mapper method %s selectKey uses unknown resultType %q", method.Name, method.Statement.SelectKey.ResultType)
		}
	}
	return nil
}

func validateCallableMethod(model *PackageModel, method MethodModel) error {
	params := methodParameterMap(method)
	for _, parameter := range method.Statement.ParameterModes {
		mode := orm.NormalizeParameterMode(parameter.Mode)
		if mode == orm.ParameterModeIn {
			continue
		}
		param, ok := params[strings.TrimSpace(parameter.Name)]
		if !ok {
			return fmt.Errorf("goark-orm: mapper method %s OUT parameter %q has no matching method parameter", method.Name, parameter.Name)
		}
		if !isPointerType(param.Type) {
			return fmt.Errorf("goark-orm: mapper method %s OUT parameter %q requires pointer parameter", method.Name, parameter.Name)
		}
	}
	for _, resultSet := range method.Statement.ResultSets {
		param, ok := params[strings.TrimSpace(resultSet.Name)]
		if !ok {
			return fmt.Errorf("goark-orm: mapper method %s resultSet %q has no matching method parameter", method.Name, resultSet.Name)
		}
		if !isPointerToSliceType(param.Type) {
			return fmt.Errorf("goark-orm: mapper method %s resultSet %q requires pointer to slice parameter", method.Name, resultSet.Name)
		}
		if err := validateCallableResultSetType(model, method, resultSet, param); err != nil {
			return err
		}
	}
	return nil
}

func validateCallableResultSetType(model *PackageModel, method MethodModel, resultSet orm.ResultSetMeta, param ParamModel) error {
	expected := normalizeResultType(resultSet.ResultType)
	if expected == "" {
		return nil
	}
	actual := normalizeResultType(slicePointerElementType(param.Type))
	if actual != expected {
		return fmt.Errorf("goark-orm: mapper method %s resultSet %q resultType %q does not match parameter %s %s", method.Name, resultSet.Name, resultSet.ResultType, param.Name, param.Type)
	}
	if !isScalarType(expected) && !entityExists(model, expected) {
		return fmt.Errorf("goark-orm: mapper method %s resultSet %q uses unknown resultType %q", method.Name, resultSet.Name, resultSet.ResultType)
	}
	return nil
}

func methodParameterMap(method MethodModel) map[string]ParamModel {
	out := make(map[string]ParamModel, len(method.Params))
	for _, param := range method.Params[1:] {
		out[param.Name] = param
	}
	return out
}

func isPointerType(typ string) bool {
	return strings.HasPrefix(strings.TrimSpace(typ), "*")
}

func isPointerToSliceType(typ string) bool {
	typ = strings.TrimSpace(typ)
	if !strings.HasPrefix(typ, "*") {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(typ, "*")), "[]")
}

func slicePointerElementType(typ string) string {
	typ = strings.TrimSpace(typ)
	typ = strings.TrimSpace(strings.TrimPrefix(typ, "*"))
	typ = strings.TrimSpace(strings.TrimPrefix(typ, "[]"))
	return typ
}

func isCallResultType(resultType string) bool {
	resultType = strings.TrimSpace(resultType)
	return resultType == "orm.CallResult" || resultType == "CallResult"
}

func validateParameterType(model *PackageModel, method MethodModel) error {
	parameterType := strings.TrimSpace(method.Statement.ParameterType)
	if parameterType == "" {
		return nil
	}
	normalized := normalizeTypeName(parameterType)
	if isMapParameterType(parameterType) {
		return nil
	}
	if !isScalarType(normalized) && !entityExists(model, normalized) {
		return fmt.Errorf("goark-orm: mapper method %s uses unknown parameterType %q", method.Name, parameterType)
	}
	dataParams := methodDataParams(method)
	if len(dataParams) != 1 {
		return fmt.Errorf("goark-orm: mapper method %s parameterType %q requires exactly one data parameter", method.Name, parameterType)
	}
	actual := normalizeTypeName(dataParams[0].Type)
	if actual != normalized {
		return fmt.Errorf("goark-orm: mapper method %s parameterType %q does not match method parameter %s %s", method.Name, parameterType, dataParams[0].Name, dataParams[0].Type)
	}
	return nil
}

func validateResultType(model *PackageModel, method MethodModel) error {
	resultType := strings.TrimSpace(method.Statement.ResultType)
	if resultType == "" {
		return nil
	}
	normalized := normalizeResultType(resultType)
	if !isScalarType(normalized) && !entityExists(model, normalized) {
		return fmt.Errorf("goark-orm: mapper method %s uses unknown resultType %q", method.Name, resultType)
	}
	expected := methodExpectedResultType(method)
	if expected != "" && normalized != expected {
		return fmt.Errorf("goark-orm: mapper method %s resultType %q does not match method result %s", method.Name, resultType, method.ResultType)
	}
	return nil
}

func methodExpectedResultType(method MethodModel) string {
	if itemType, ok := pageResultTypeArg(method.ResultType); ok {
		return normalizeResultType(itemType)
	}
	if itemType, ok := cursorResultTypeArg(method.ResultType); ok {
		return normalizeResultType(itemType)
	}
	if _, itemType, ok := resultHandlerParam(method); ok {
		return normalizeResultType(itemType)
	}
	return normalizeResultType(method.ResultType)
}

func validateResultMapTypes(model *PackageModel, mapper MapperModel) error {
	for _, resultMap := range mapper.ResultMaps {
		if err := validateResultObjectType(model, mapper.TypeName, "resultMap "+resultMap.ID, resultMap.TypeName); err != nil {
			return err
		}
		for _, association := range resultMap.Associations {
			if err := validateAssociationType(model, mapper.TypeName, resultMap.ID, association); err != nil {
				return err
			}
		}
		for _, collection := range resultMap.Collections {
			if err := validateCollectionType(model, mapper.TypeName, resultMap.ID, collection); err != nil {
				return err
			}
		}
		if err := validateDiscriminatorTypes(model, mapper, resultMap); err != nil {
			return err
		}
	}
	return nil
}

func validateDiscriminatorTypes(model *PackageModel, mapper MapperModel, resultMap orm.ResultMapMeta) error {
	for _, item := range resultMap.Discriminator.Cases {
		location := "discriminator case " + item.Value + " on resultMap " + resultMap.ID
		if item.ResultMap != "" && !resultMapExists(mapper, localXMLResultMapID(mapper.Namespace, item.ResultMap)) {
			return fmt.Errorf("goark-orm: mapper %s %s references missing resultMap %q", mapper.TypeName, location, item.ResultMap)
		}
		if err := validateResultObjectType(model, mapper.TypeName, location, item.ResultType); err != nil {
			return err
		}
		for _, association := range item.Associations {
			if err := validateAssociationType(model, mapper.TypeName, resultMap.ID, association); err != nil {
				return err
			}
		}
		for _, collection := range item.Collections {
			if err := validateCollectionType(model, mapper.TypeName, resultMap.ID, collection); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAssociationType(model *PackageModel, mapperName string, resultMapID string, association orm.ResultAssociationMeta) error {
	if err := validateResultObjectType(model, mapperName, "association "+association.Property+" on resultMap "+resultMapID, association.TypeName); err != nil {
		return err
	}
	for _, child := range association.Associations {
		if err := validateAssociationType(model, mapperName, resultMapID, child); err != nil {
			return err
		}
	}
	for _, child := range association.Collections {
		if err := validateCollectionType(model, mapperName, resultMapID, child); err != nil {
			return err
		}
	}
	return nil
}

func validateCollectionType(model *PackageModel, mapperName string, resultMapID string, collection orm.ResultCollectionMeta) error {
	if err := validateResultObjectType(model, mapperName, "collection "+collection.Property+" on resultMap "+resultMapID, collection.TypeName); err != nil {
		return err
	}
	for _, child := range collection.Associations {
		if err := validateAssociationType(model, mapperName, resultMapID, child); err != nil {
			return err
		}
	}
	for _, child := range collection.Collections {
		if err := validateCollectionType(model, mapperName, resultMapID, child); err != nil {
			return err
		}
	}
	return nil
}

func validateResultObjectType(model *PackageModel, mapperName string, location string, typeName string) error {
	typeName = normalizeTypeName(typeName)
	if typeName == "" || isScalarType(typeName) || entityExists(model, typeName) {
		return nil
	}
	return fmt.Errorf("goark-orm: mapper %s %s uses unknown type %q", mapperName, location, typeName)
}

func entityExists(model *PackageModel, typeName string) bool {
	typeName = normalizeTypeName(typeName)
	for _, entity := range model.Entities {
		if entity.TypeName == typeName {
			return true
		}
	}
	return false
}

func isScalarType(typeName string) bool {
	switch normalizeTypeName(typeName) {
	case "bool",
		"byte",
		"rune",
		"string",
		"int",
		"int8",
		"int16",
		"int32",
		"int64",
		"uint",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"uintptr",
		"float32",
		"float64",
		"Time",
		"any",
		"interface{}":
		return true
	default:
		return false
	}
}

func isMapParameterType(typeName string) bool {
	normalized := strings.ReplaceAll(strings.TrimSpace(typeName), " ", "")
	switch normalized {
	case "orm.NamedArgs", "NamedArgs", "map[string]any", "map[string]interface{}":
		return true
	default:
		return false
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
		root, valid := parameterRootName(name)
		if !valid {
			return fmt.Errorf("goark-orm: mapper method %s SQL parameter %q is invalid", method.Name, name)
		}
		if _, ok := available[name]; ok {
			continue
		}
		if _, ok := available[root]; !ok {
			return fmt.Errorf("goark-orm: mapper method %s SQL parameter %q has no matching method parameter or entity field", method.Name, name)
		}
	}
	return nil
}

func availableParameters(model *PackageModel, method MethodModel) map[string]struct{} {
	out := make(map[string]struct{})
	dataParams := methodBindableParams(method)
	for index, param := range dataParams {
		out[param.Name] = struct{}{}
		out[fmt.Sprintf("param%d", index+1)] = struct{}{}
	}
	if len(dataParams) != 1 {
		return out
	}
	out["_parameter"] = struct{}{}
	if isCollectionParameterType(dataParams[0].Type) {
		out["collection"] = struct{}{}
		out["list"] = struct{}{}
		out["array"] = struct{}{}
	}
	entityName := normalizeTypeName(dataParams[0].Type)
	for _, entity := range model.Entities {
		if entity.TypeName != entityName {
			continue
		}
		for _, column := range entity.Columns {
			out[column.FieldName] = struct{}{}
			if alias := propertyAlias(column.FieldName); alias != "" {
				out[alias] = struct{}{}
			}
		}
	}
	return out
}

func methodDataParams(method MethodModel) []ParamModel {
	out := make([]ParamModel, 0, len(method.Params))
	for index, param := range method.Params {
		if index == 0 || isPageRequestType(param.Type) || isResultHandlerType(param.Type) {
			continue
		}
		out = append(out, param)
	}
	return out
}

func methodBindableParams(method MethodModel) []ParamModel {
	dataParams := methodDataParams(method)
	if method.Statement.Command != orm.StatementCommandCall || len(method.Statement.ResultSets) == 0 {
		return dataParams
	}
	resultSetParams := make(map[string]struct{}, len(method.Statement.ResultSets))
	for _, resultSet := range method.Statement.ResultSets {
		name := strings.TrimSpace(resultSet.Name)
		if name != "" {
			resultSetParams[name] = struct{}{}
		}
	}
	out := dataParams[:0]
	for _, param := range dataParams {
		if _, ok := resultSetParams[param.Name]; ok {
			continue
		}
		out = append(out, param)
	}
	return out
}

func pageRequestParam(method MethodModel) (ParamModel, bool) {
	for _, param := range method.Params[1:] {
		if isPageRequestType(param.Type) {
			return param, true
		}
	}
	return ParamModel{}, false
}

func isPageRequestType(typ string) bool {
	typ = strings.TrimSpace(typ)
	return typ == "orm.PageRequest" || typ == "PageRequest"
}

func resultHandlerParam(method MethodModel) (ParamModel, string, bool) {
	for _, param := range method.Params[1:] {
		if itemType, ok := resultHandlerTypeArg(param.Type); ok {
			return param, itemType, true
		}
	}
	return ParamModel{}, "", false
}

func isResultHandlerType(typ string) bool {
	_, ok := resultHandlerTypeArg(typ)
	return ok
}

func resultHandlerTypeArg(typ string) (string, bool) {
	typ = strings.TrimSpace(typ)
	for _, prefix := range []string{"orm.ResultHandler[", "ResultHandler["} {
		if strings.HasPrefix(typ, prefix) && strings.HasSuffix(typ, "]") {
			itemType := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(typ, prefix), "]"))
			return itemType, itemType != ""
		}
	}
	return "", false
}

func pageResultTypeArg(typ string) (string, bool) {
	typ = strings.TrimSpace(typ)
	for _, prefix := range []string{"orm.Page[", "Page["} {
		if strings.HasPrefix(typ, prefix) && strings.HasSuffix(typ, "]") {
			itemType := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(typ, prefix), "]"))
			return itemType, itemType != ""
		}
	}
	return "", false
}

func cursorResultTypeArg(typ string) (string, bool) {
	typ = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(typ), "*"))
	for _, prefix := range []string{"orm.Cursor[", "Cursor["} {
		if strings.HasPrefix(typ, prefix) && strings.HasSuffix(typ, "]") {
			itemType := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(typ, prefix), "]"))
			return itemType, itemType != ""
		}
	}
	return "", false
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

func parseInterceptorIgnores(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	return uniqueSorted(parts)
}

func parameterRootName(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || !isIdentifierStart(rune(raw[0])) {
		return "", false
	}
	index := 1
	for index < len(raw) && isIdentifierPart(rune(raw[index])) {
		index++
	}
	if !validParameterPathTail(raw[index:]) {
		return "", false
	}
	return raw[:index], true
}

func validParameterPathTail(raw string) bool {
	for index := 0; index < len(raw); {
		switch raw[index] {
		case '.':
			index++
			if index >= len(raw) || !isIdentifierStart(rune(raw[index])) {
				return false
			}
			index++
			for index < len(raw) && isIdentifierPart(rune(raw[index])) {
				index++
			}
		case '[':
			end := strings.IndexByte(raw[index:], ']')
			if end < 0 {
				return false
			}
			end += index
			content := strings.TrimSpace(raw[index+1 : end])
			if content == "" {
				return false
			}
			if content[0] == '\'' || content[0] == '"' {
				if len(content) < 2 || content[len(content)-1] != content[0] {
					return false
				}
			} else {
				for pos, r := range content {
					if pos == 0 && r == '-' {
						return false
					}
					if !isIdentifierPart(r) {
						return false
					}
				}
			}
			index = end + 1
		default:
			return false
		}
	}
	return true
}

func isCollectionParameterType(typ string) bool {
	typ = strings.TrimSpace(typ)
	return strings.HasPrefix(typ, "[]") || strings.HasPrefix(typ, "[")
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
		if err := validateResultMapTypes(model, mapper); err != nil {
			return err
		}
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
