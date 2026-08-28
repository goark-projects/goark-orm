package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"sync"
)

type nestedSelectPlan struct {
	property   string
	fieldIndex []int
	column     string
	selectName string
	collection bool
	fetchType  string
	lazy       bool
}

type nestedSelectColumnMapping struct {
	parameter string
	source    string
}

type nestedSelectLoadContext struct {
	mu     sync.Mutex
	values map[string]reflect.Value
}

func newNestedSelectLoadContext() *nestedSelectLoadContext {
	return &nestedSelectLoadContext{values: make(map[string]reflect.Value)}
}

func resultMapHasNestedSelects(resultMap ResultMapMeta) bool {
	if len(resultNestedSelectPlans(nil, resultMap)) > 0 {
		return true
	}
	for _, item := range resultMap.Discriminator.Cases {
		if len(resultNestedSelectPlans(nil, discriminatorCaseResultMap(item))) > 0 {
			return true
		}
	}
	return false
}

func resultNestedSelectPlans(rootType reflect.Type, resultMap ResultMapMeta) []nestedSelectPlan {
	plans := make([]nestedSelectPlan, 0, len(resultMap.Associations)+len(resultMap.Collections))
	for _, association := range resultMap.Associations {
		selectName := strings.TrimSpace(association.Select)
		if selectName == "" {
			continue
		}
		plan := nestedSelectPlan{
			property:   association.Property,
			column:     strings.TrimSpace(association.Column),
			selectName: selectName,
			fetchType:  strings.TrimSpace(association.FetchType),
		}
		if rootType != nil {
			field, ok := rootType.FieldByName(association.Property)
			if !ok || field.PkgPath != "" {
				continue
			}
			plan.lazy = isLazyFetchType(plan.fetchType) || isLazyLoaderFieldType(field.Type)
			if !plan.lazy && !isNestedAssociationFieldType(field.Type) {
				continue
			}
			plan.fieldIndex = field.Index
		}
		plans = append(plans, plan)
	}
	for _, collection := range resultMap.Collections {
		selectName := strings.TrimSpace(collection.Select)
		if selectName == "" {
			continue
		}
		plan := nestedSelectPlan{
			property:   collection.Property,
			column:     strings.TrimSpace(collection.Column),
			selectName: selectName,
			collection: true,
			fetchType:  strings.TrimSpace(collection.FetchType),
		}
		if rootType != nil {
			field, ok := rootType.FieldByName(collection.Property)
			if !ok || field.PkgPath != "" {
				continue
			}
			plan.lazy = isLazyFetchType(plan.fetchType) || isLazyLoaderFieldType(field.Type)
			if !plan.lazy && field.Type.Kind() != reflect.Slice {
				continue
			}
			plan.fieldIndex = field.Index
		}
		plans = append(plans, plan)
	}
	return plans
}

func (s *SQLSession) applyNestedSelects(ctx context.Context, parent StatementMeta, resultMap ResultMapMeta, target reflect.Value) error {
	if s == nil || !target.IsValid() {
		return nil
	}
	target, ok := nestedSelectRootValue(target)
	if !ok {
		return nil
	}
	switch target.Kind() {
	case reflect.Slice:
		loadContext := newNestedSelectLoadContext()
		elementType := target.Type().Elem()
		if elementType.Kind() == reflect.Pointer {
			elementType = elementType.Elem()
		}
		if elementType.Kind() != reflect.Struct {
			return nil
		}
		plans := resultNestedSelectPlans(elementType, resultMap)
		if len(plans) == 0 {
			return nil
		}
		for index := 0; index < target.Len(); index++ {
			root := rootValueAt(target, index)
			if err := s.applyNestedSelectPlans(ctx, parent, root, plans, loadContext); err != nil {
				return err
			}
		}
	case reflect.Struct:
		loadContext := newNestedSelectLoadContext()
		plans := resultNestedSelectPlans(target.Type(), resultMap)
		if len(plans) == 0 {
			return nil
		}
		return s.applyNestedSelectPlans(ctx, parent, target, plans, loadContext)
	}
	return nil
}

func nestedSelectRootValue(value reflect.Value) (reflect.Value, bool) {
	for value.IsValid() && (value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer) {
		if value.IsNil() {
			return reflect.Value{}, false
		}
		value = value.Elem()
	}
	return value, value.IsValid()
}

func (s *SQLSession) applyNestedSelectPlans(ctx context.Context, parent StatementMeta, root reflect.Value, plans []nestedSelectPlan, loadContext *nestedSelectLoadContext) error {
	if root.Kind() != reflect.Struct {
		return nil
	}
	for _, plan := range plans {
		statement, err := s.lookupNestedSelectStatement(parent, plan.selectName)
		if err != nil {
			return err
		}
		args, ok, err := s.nestedSelectArgs(parent, statement, root, plan.column)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		field, ok := fieldByIndexAlloc(root, plan.fieldIndex)
		if !ok || !field.IsValid() || !field.CanSet() {
			continue
		}
		if plan.lazy {
			applied, err := s.applyNestedLazy(ctx, statement, args, field, plan, loadContext)
			if err != nil {
				return err
			}
			if applied {
				continue
			}
		}
		if plan.collection {
			if err := s.applyNestedCollection(ctx, statement, args, field, loadContext); err != nil {
				return err
			}
			continue
		}
		if err := s.applyNestedAssociation(ctx, statement, args, field, loadContext); err != nil {
			return err
		}
	}
	return nil
}

func isLazyFetchType(fetchType string) bool {
	return strings.EqualFold(strings.TrimSpace(fetchType), "lazy")
}

func isLazyLoaderFieldType(typ reflect.Type) bool {
	if typ == nil || typ.Kind() != reflect.Struct {
		return false
	}
	return reflect.PointerTo(typ).Implements(reflect.TypeFor[lazyLoaderTarget]())
}

func isNestedAssociationFieldType(typ reflect.Type) bool {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ.Kind() == reflect.Struct
}

func (s *SQLSession) applyNestedLazy(ctx context.Context, statement StatementMeta, args NamedArgs, field reflect.Value, plan nestedSelectPlan, loadContext *nestedSelectLoadContext) (bool, error) {
	if !field.CanAddr() || !isLazyLoaderFieldType(field.Type()) {
		return false, nil
	}
	target, ok := field.Addr().Interface().(lazyLoaderTarget)
	if !ok {
		return false, nil
	}
	loadType, ok := lazyFieldLoadType(field.Type())
	if !ok {
		return false, fmt.Errorf("goark-orm: lazy field %s missing Load method", field.Type())
	}
	if plan.collection {
		if loadType.Kind() != reflect.Slice {
			return false, fmt.Errorf("goark-orm: lazy collection %s must load slice", field.Type())
		}
	} else if !isNestedAssociationFieldType(loadType) {
		return false, fmt.Errorf("goark-orm: lazy association %s must load struct or pointer to struct", field.Type())
	}
	if loadContext == nil {
		loadContext = newNestedSelectLoadContext()
	}
	argsCopy := copyNamedArgs(args)
	loader := func(loadCtx context.Context) (any, error) {
		if loadCtx == nil {
			return nil, fmt.Errorf("goark-orm: context is nil")
		}
		if err := loadCtx.Err(); err != nil {
			return nil, err
		}
		cacheKey := nestedSelectCacheKey(statement, argsCopy, loadType)
		if cached, ok := loadContext.value(cacheKey); ok {
			return cached.Interface(), nil
		}
		var value reflect.Value
		var err error
		if plan.collection {
			value, err = s.loadNestedCollectionValue(loadCtx, statement, argsCopy, loadType)
		} else {
			value, err = s.loadNestedAssociationValue(loadCtx, statement, argsCopy, loadType)
		}
		if err != nil {
			return nil, err
		}
		loadContext.storeValue(cacheKey, value)
		return value.Interface(), nil
	}
	if err := target.setAnyLoader(loader); err != nil {
		return false, err
	}
	_ = ctx
	return true, nil
}

func lazyFieldLoadType(typ reflect.Type) (reflect.Type, bool) {
	method, ok := reflect.PointerTo(typ).MethodByName("Load")
	if !ok || method.Type.NumOut() != 2 {
		return nil, false
	}
	errorType := reflect.TypeFor[error]()
	if !method.Type.Out(1).Implements(errorType) {
		return nil, false
	}
	return method.Type.Out(0), true
}

func (s *SQLSession) loadNestedAssociationValue(ctx context.Context, statement StatementMeta, args NamedArgs, targetType reflect.Type) (reflect.Value, error) {
	if targetType.Kind() == reflect.Pointer {
		if targetType.Elem().Kind() != reflect.Struct {
			return reflect.Value{}, fmt.Errorf("goark-orm: lazy association %s must load struct or pointer to struct", targetType)
		}
		dest := reflect.New(targetType.Elem())
		if err := s.QueryOneStatement(ctx, statement, args, dest.Interface()); err != nil {
			if err == sql.ErrNoRows {
				return reflect.Zero(targetType), nil
			}
			return reflect.Value{}, err
		}
		return dest, nil
	}
	if targetType.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("goark-orm: lazy association %s must load struct or pointer to struct", targetType)
	}
	dest := reflect.New(targetType)
	if err := s.QueryOneStatement(ctx, statement, args, dest.Interface()); err != nil {
		if err == sql.ErrNoRows {
			return reflect.Zero(targetType), nil
		}
		return reflect.Value{}, err
	}
	return dest.Elem(), nil
}

func (s *SQLSession) loadNestedCollectionValue(ctx context.Context, statement StatementMeta, args NamedArgs, targetType reflect.Type) (reflect.Value, error) {
	if targetType.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("goark-orm: lazy collection %s must load slice", targetType)
	}
	dest := reflect.New(targetType)
	if err := s.QueryStatement(ctx, statement, args, dest.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return dest.Elem(), nil
}

func (s *SQLSession) nestedSelectArgs(parent StatementMeta, statement StatementMeta, root reflect.Value, column string) (NamedArgs, bool, error) {
	mappings, composite, err := parseNestedSelectCompositeColumn(column)
	if err != nil {
		return nil, false, err
	}
	if !composite {
		arg, ok, err := s.nestedSelectColumnValue(parent, root, column)
		if err != nil || !ok || isNilValue(arg) {
			return nil, false, err
		}
		return nestedSelectArgs(statement, column, arg), true, nil
	}
	args := make(NamedArgs, len(mappings)+3)
	allNil := true
	for _, mapping := range mappings {
		value, ok, err := s.nestedSelectColumnValue(parent, root, mapping.source)
		if err != nil {
			return nil, false, err
		}
		if !ok {
			return nil, false, fmt.Errorf("goark-orm: nested select column source %q on statement %s cannot be resolved", mapping.source, parent.FullName)
		}
		args[mapping.parameter] = value
		if !isNilValue(value) {
			allNil = false
		}
	}
	if allNil {
		return nil, false, nil
	}
	parameterObject := copyNamedArgs(args)
	args["_parameter"] = parameterObject
	args["param1"] = parameterObject
	args["value"] = parameterObject
	return args, true, nil
}

func (s *SQLSession) nestedSelectColumnValue(parent StatementMeta, root reflect.Value, column string) (any, bool, error) {
	column = strings.TrimSpace(column)
	if column == "" {
		return root.Interface(), true, nil
	}
	bindings := s.columnBindings(parent, root.Type())
	if binding, ok := bindings[normalizeColumnKey(column)]; ok {
		field, ok := fieldByIndexAlloc(root, binding.index)
		if !ok || !field.IsValid() || !field.CanInterface() {
			return nil, false, nil
		}
		return field.Interface(), true, nil
	}
	field, ok := exportedFieldByProperty(root, column)
	if !ok || !field.CanInterface() {
		return nil, false, nil
	}
	return field.Interface(), true, nil
}

func (s *SQLSession) lookupNestedSelectStatement(parent StatementMeta, selectName string) (StatementMeta, error) {
	selectName = strings.TrimSpace(selectName)
	if selectName == "" {
		return StatementMeta{}, fmt.Errorf("goark-orm: nested select is empty on statement %s", parent.FullName)
	}
	if statement, ok := s.registry.Statement(selectName); ok {
		return statement, nil
	}
	localName := parent.Namespace + "." + selectName
	if statement, ok := s.registry.Statement(localName); ok {
		return statement, nil
	}
	return StatementMeta{}, fmt.Errorf("goark-orm: nested select %q on statement %s is not registered", selectName, parent.FullName)
}

func nestedSelectArgs(statement StatementMeta, column string, value any) NamedArgs {
	args := NamedArgs{
		"_parameter": value,
		"param1":     value,
		"value":      value,
	}
	for _, name := range columnParameterAliases(column) {
		args[name] = value
	}
	if len(statement.Parameters) == 1 {
		args[statement.Parameters[0]] = value
	}
	if len(statement.Parameters) == 0 {
		args["id"] = value
	}
	return args
}

func parseNestedSelectCompositeColumn(column string) ([]nestedSelectColumnMapping, bool, error) {
	column = strings.TrimSpace(column)
	if column == "" || !strings.HasPrefix(column, "{") {
		return nil, false, nil
	}
	if !strings.HasSuffix(column, "}") {
		return nil, true, fmt.Errorf("goark-orm: nested select composite column %q is not closed", column)
	}
	body := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(column, "{"), "}"))
	if body == "" {
		return nil, true, fmt.Errorf("goark-orm: nested select composite column %q is empty", column)
	}
	parts := strings.Split(body, ",")
	mappings := make([]nestedSelectColumnMapping, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		left, right, ok := strings.Cut(part, "=")
		if !ok {
			return nil, true, fmt.Errorf("goark-orm: nested select composite column item %q must be parameter=column", strings.TrimSpace(part))
		}
		parameter := strings.TrimSpace(left)
		source := strings.TrimSpace(right)
		if !validIdentifierPart(parameter) {
			return nil, true, fmt.Errorf("goark-orm: nested select composite parameter %q is invalid", parameter)
		}
		if source == "" {
			return nil, true, fmt.Errorf("goark-orm: nested select composite column source for %q is empty", parameter)
		}
		if _, ok := seen[parameter]; ok {
			return nil, true, fmt.Errorf("goark-orm: nested select composite parameter %q is duplicated", parameter)
		}
		seen[parameter] = struct{}{}
		mappings = append(mappings, nestedSelectColumnMapping{parameter: parameter, source: source})
	}
	return mappings, true, nil
}

func columnParameterAliases(column string) []string {
	column = strings.TrimSpace(column)
	if column == "" {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, 3)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || !validIdentifierPart(value) {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	add(column)
	add(underscoreToLowerCamel(column))
	if strings.HasSuffix(column, "_id") {
		add("id")
	}
	return out
}

func underscoreToLowerCamel(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "_")
	if len(parts) == 0 {
		return ""
	}
	var builder strings.Builder
	for index, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if index == 0 {
			builder.WriteString(strings.ToLower(part))
			continue
		}
		if strings.EqualFold(part, "id") {
			builder.WriteString("ID")
			continue
		}
		builder.WriteString(strings.ToUpper(part[:1]))
		if len(part) > 1 {
			builder.WriteString(strings.ToLower(part[1:]))
		}
	}
	return builder.String()
}

func (s *SQLSession) applyNestedAssociation(ctx context.Context, statement StatementMeta, args NamedArgs, field reflect.Value, loadContext *nestedSelectLoadContext) error {
	for field.Kind() == reflect.Interface {
		if field.IsNil() {
			return nil
		}
		field = field.Elem()
	}
	cacheKey := nestedSelectCacheKey(statement, args, field.Type())
	if ok, err := loadContext.assign(cacheKey, field); err != nil || ok {
		return err
	}
	if field.Kind() == reflect.Pointer {
		if field.Type().Elem().Kind() != reflect.Struct {
			return fmt.Errorf("goark-orm: nested association %s must be struct or pointer to struct", field.Type())
		}
		dest := reflect.New(field.Type().Elem())
		if err := s.QueryOneStatement(ctx, statement, args, dest.Interface()); err != nil {
			if err == sql.ErrNoRows {
				field.Set(reflect.Zero(field.Type()))
				loadContext.store(cacheKey, field)
				return nil
			}
			return err
		}
		field.Set(dest)
		loadContext.store(cacheKey, field)
		return nil
	}
	if field.Kind() != reflect.Struct || !field.CanAddr() {
		return fmt.Errorf("goark-orm: nested association %s must be addressable struct", field.Type())
	}
	if err := s.QueryOneStatement(ctx, statement, args, field.Addr().Interface()); err != nil {
		if err == sql.ErrNoRows {
			field.Set(reflect.Zero(field.Type()))
			loadContext.store(cacheKey, field)
			return nil
		}
		return err
	}
	loadContext.store(cacheKey, field)
	return nil
}

func (s *SQLSession) applyNestedCollection(ctx context.Context, statement StatementMeta, args NamedArgs, field reflect.Value, loadContext *nestedSelectLoadContext) error {
	for field.Kind() == reflect.Interface {
		if field.IsNil() {
			return nil
		}
		field = field.Elem()
	}
	if field.Kind() != reflect.Slice || !field.CanSet() {
		return fmt.Errorf("goark-orm: nested collection %s must be settable slice", field.Type())
	}
	cacheKey := nestedSelectCacheKey(statement, args, field.Type())
	if ok, err := loadContext.assign(cacheKey, field); err != nil || ok {
		return err
	}
	dest := reflect.New(field.Type())
	if err := s.QueryStatement(ctx, statement, args, dest.Interface()); err != nil {
		return err
	}
	field.Set(dest.Elem())
	loadContext.store(cacheKey, field)
	return nil
}

func discriminatorCaseResultMap(item ResultDiscriminatorCaseMeta) ResultMapMeta {
	return ResultMapMeta{
		ID:           item.ResultMap,
		TypeName:     item.ResultType,
		Fields:       item.Fields,
		Associations: item.Associations,
		Collections:  item.Collections,
	}
}

func (c *nestedSelectLoadContext) assign(key string, field reflect.Value) (bool, error) {
	if c == nil || c.values == nil {
		return false, nil
	}
	c.mu.Lock()
	value, ok := c.values[key]
	c.mu.Unlock()
	if !ok {
		return false, nil
	}
	if err := assignReflectValue(field, cloneReflectValue(value)); err != nil {
		return false, err
	}
	return true, nil
}

func (c *nestedSelectLoadContext) store(key string, field reflect.Value) {
	if c == nil || c.values == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = cloneReflectValue(field)
}

func (c *nestedSelectLoadContext) value(key string) (reflect.Value, bool) {
	if c == nil || c.values == nil {
		return reflect.Value{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.values[key]
	if !ok {
		return reflect.Value{}, false
	}
	return cloneReflectValue(value), true
}

func (c *nestedSelectLoadContext) storeValue(key string, value reflect.Value) {
	if c == nil || c.values == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = cloneReflectValue(value)
}

func assignReflectValue(target reflect.Value, value reflect.Value) error {
	if !target.IsValid() || !target.CanSet() {
		return nil
	}
	if !value.IsValid() {
		target.Set(reflect.Zero(target.Type()))
		return nil
	}
	if value.Type().AssignableTo(target.Type()) {
		target.Set(value)
		return nil
	}
	if value.Type().ConvertibleTo(target.Type()) {
		target.Set(value.Convert(target.Type()))
		return nil
	}
	return fmt.Errorf("goark-orm: nested select cache value %s cannot assign to %s", value.Type(), target.Type())
}

func nestedSelectCacheKey(statement StatementMeta, args NamedArgs, targetType reflect.Type) string {
	var builder strings.Builder
	builder.WriteString(statement.FullName)
	builder.WriteByte(0)
	if targetType != nil {
		builder.WriteString(targetType.String())
	}
	keys := nestedSelectCacheParameterNames(statement, args)
	for _, key := range keys {
		value, _, _ := resolveNamedArg(args, key)
		builder.WriteByte(0)
		builder.WriteString(key)
		builder.WriteByte('=')
		_, _ = fmt.Fprintf(&builder, "%T:%#v", value, value)
	}
	return builder.String()
}

func nestedSelectCacheParameterNames(statement StatementMeta, args NamedArgs) []string {
	if len(statement.Parameters) > 0 {
		keys := append([]string(nil), statement.Parameters...)
		sort.Strings(keys)
		return keys
	}
	keys := make([]string, 0, len(args))
	for key := range args {
		if key == "_parameter" || key == "param1" || key == "value" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
