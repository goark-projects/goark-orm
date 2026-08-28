package runtime

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// ValidateRegistry 对注册表执行启动前静态一致性校验。
func ValidateRegistry(registry *Registry) error {
	if registry == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	return registry.Validate()
}

// Validate 对注册表执行启动前静态一致性校验。
func (r *Registry) Validate() error {
	if r == nil {
		return registryErrorf("registry", "", "registry is nil")
	}
	validator := registryValidator{snapshot: r.validationSnapshot()}
	return validator.validate()
}

type registryValidationSnapshot struct {
	entities   map[string]EntityMeta
	mappers    map[string]MapperMeta
	statements map[string]StatementMeta
	caches     map[string]Cache
	cacheRefs  map[string]string
	handlers   map[string]TypeHandler
	providers  map[string]SQLProviderDescriptor
}

type registryValidator struct {
	snapshot registryValidationSnapshot
	errs     []error
}

func (r *Registry) validationSnapshot() registryValidationSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := registryValidationSnapshot{
		entities:   make(map[string]EntityMeta, len(r.entities)),
		mappers:    make(map[string]MapperMeta, len(r.mappers)),
		statements: make(map[string]StatementMeta, len(r.statements)),
		caches:     make(map[string]Cache, len(r.caches)),
		cacheRefs:  make(map[string]string, len(r.cacheRefs)),
		handlers:   make(map[string]TypeHandler, len(r.handlers)),
		providers:  make(map[string]SQLProviderDescriptor, len(r.providers)),
	}
	for name, entity := range r.entities {
		snapshot.entities[name] = copyEntityMeta(entity)
	}
	for namespace, mapper := range r.mappers {
		snapshot.mappers[namespace] = copyMapperMeta(mapper)
	}
	for name, statement := range r.statements {
		snapshot.statements[name] = statement
	}
	for namespace, cache := range r.caches {
		snapshot.caches[namespace] = cache
	}
	for namespace, ref := range r.cacheRefs {
		snapshot.cacheRefs[namespace] = ref
	}
	for name, handler := range r.handlers {
		snapshot.handlers[name] = handler
	}
	for name, provider := range r.providers {
		snapshot.providers[name] = copySQLProviderDescriptor(provider)
	}
	return snapshot
}

func (v *registryValidator) validate() error {
	v.validateEntities()
	v.validateSQLProviderDescriptors()
	for _, namespace := range sortedValidationKeys(v.snapshot.mappers) {
		v.validateMapper(v.snapshot.mappers[namespace])
	}
	if len(v.errs) == 0 {
		return nil
	}
	return errors.Join(v.errs...)
}

func (v *registryValidator) validateEntities() {
	for _, typeName := range sortedValidationKeys(v.snapshot.entities) {
		entity := v.snapshot.entities[typeName]
		for _, column := range entity.Columns {
			v.validateTypeHandler(column.TypeHandler, "entity", entity.TypeName, column.FieldName)
		}
	}
}

func (v *registryValidator) validateSQLProviderDescriptors() {
	for _, name := range sortedValidationKeys(v.snapshot.providers) {
		descriptor := v.snapshot.providers[name]
		providerName := strings.TrimSpace(descriptor.Name)
		if providerName == "" {
			v.add("SQL provider", name, "SQL provider name is required")
			providerName = name
		}
		if descriptor.Provider == nil {
			v.add("SQL provider", providerName, "SQL provider %q is nil", providerName)
		}
		for _, statementName := range descriptor.Statements {
			statementName = strings.TrimSpace(statementName)
			if statementName == "" {
				v.add("SQL provider", providerName, "SQL provider %q has empty statement constraint", providerName)
				continue
			}
			if _, ok := v.snapshot.statements[statementName]; !ok {
				v.add("SQL provider", providerName, "SQL provider %q references unknown statement %s", providerName, statementName)
			}
		}
		for _, command := range descriptor.Commands {
			if !validStatementCommand(command) {
				v.add("SQL provider", providerName, "SQL provider %q uses invalid command %q", providerName, command)
			}
		}
	}
}

func (v *registryValidator) validateMapper(mapper MapperMeta) {
	namespace := strings.TrimSpace(mapper.Namespace)
	if namespace == "" {
		v.add("mapper", mapper.TypeName, "mapper %s namespace is required", mapper.TypeName)
		return
	}
	resultMaps := v.indexResultMaps(mapper)
	if mapper.Cache.Enabled && strings.TrimSpace(mapper.Cache.RefNamespace) != "" {
		v.validateCacheRef(namespace)
	}
	for _, resultMap := range mapper.ResultMaps {
		v.validateResultMap(mapper, resultMaps, resultMap)
	}
	v.validateResultMapReferenceCycles(mapper, resultMaps)
	for _, statement := range mapper.Statements {
		v.validateStatement(mapper, resultMaps, statement)
	}
}

func (v *registryValidator) indexResultMaps(mapper MapperMeta) map[string]ResultMapMeta {
	resultMaps := make(map[string]ResultMapMeta, len(mapper.ResultMaps))
	for _, resultMap := range mapper.ResultMaps {
		id := strings.TrimSpace(resultMap.ID)
		if id == "" {
			v.add("resultMap", mapper.Namespace, "mapper %s has resultMap without id", mapper.Namespace)
			continue
		}
		if _, exists := resultMaps[id]; exists {
			v.add("resultMap", resultMapQualifiedName(mapper.Namespace, id), "duplicate resultMap %s", resultMapQualifiedName(mapper.Namespace, id))
			continue
		}
		resultMaps[id] = resultMap
	}
	return resultMaps
}

func (v *registryValidator) validateStatement(mapper MapperMeta, resultMaps map[string]ResultMapMeta, statement StatementMeta) {
	fullName := validationStatementName(mapper.Namespace, statement)
	if strings.TrimSpace(statement.Namespace) == "" {
		v.add("statement", fullName, "statement %s namespace is required", fullName)
	} else if strings.TrimSpace(statement.Namespace) != strings.TrimSpace(mapper.Namespace) {
		v.add("statement", fullName, "statement %s namespace %q does not match mapper namespace %q", fullName, statement.Namespace, mapper.Namespace)
	}
	if !validStatementCommand(statement.Command) {
		v.add("statement", fullName, "statement %s command %q is invalid", fullName, statement.Command)
	}
	if !statementHasSQLSource(statement) {
		v.add("statement", fullName, "statement %s requires SQL, DynamicSQL or SQL provider", fullName)
	}
	if providerName := strings.TrimSpace(statement.Provider); providerName != "" {
		descriptor, ok := v.snapshot.providers[providerName]
		if !ok {
			v.add("SQL provider", providerName, "SQL provider %q for statement %s is not registered", providerName, fullName)
		} else if err := descriptor.ValidateStatement(statement); err != nil {
			v.addCause("SQL provider", providerName, fmt.Sprintf("SQL provider %q is not valid for statement %s", providerName, fullName), err)
		}
	}
	if strings.TrimSpace(statement.ResultMap) != "" {
		v.validateResultMapReference(mapper.Namespace, resultMaps, statement.ResultMap, "statement", fullName)
		v.validateStatementResultSetMappings(mapper.Namespace, resultMaps, statement, fullName)
	}
	for _, resultSet := range statement.ResultSets {
		if strings.TrimSpace(resultSet.ResultMap) != "" {
			v.validateResultMapReference(mapper.Namespace, resultMaps, resultSet.ResultMap, "statement resultSet", fullName)
		}
	}
	for _, parameter := range statement.ParameterModes {
		if strings.TrimSpace(parameter.Name) == "" {
			v.add("parameter", fullName, "statement %s has call parameter without name", fullName)
		}
		v.validateTypeHandler(parameter.TypeHandler, "parameter", fullName, parameter.Name)
	}
	v.validateSelectKey(fullName, statement)
}

func (v *registryValidator) validateResultMap(mapper MapperMeta, resultMaps map[string]ResultMapMeta, resultMap ResultMapMeta) {
	name := resultMapQualifiedName(mapper.Namespace, resultMap.ID)
	if strings.TrimSpace(resultMap.TypeName) == "" {
		v.add("resultMap", name, "resultMap %s type name is required", name)
	}
	if strings.TrimSpace(resultMap.Extends) != "" {
		v.validateResultMapReference(mapper.Namespace, resultMaps, resultMap.Extends, "resultMap extends", name)
	}
	for _, arg := range resultMap.Constructor.Args {
		property := firstNonEmpty(arg.Property, arg.Name)
		v.validateTypeHandler(arg.TypeHandler, "resultMap constructor", name, property)
	}
	for _, field := range resultMap.Fields {
		v.validateTypeHandler(field.TypeHandler, "resultMap field", name, field.Property)
	}
	v.validateAssociations(mapper.Namespace, name, resultMap.Associations)
	v.validateCollections(mapper.Namespace, name, resultMap.Collections)
	v.validateDiscriminator(mapper.Namespace, resultMaps, name, resultMap.Discriminator)
}

func (v *registryValidator) validateDiscriminator(namespace string, resultMaps map[string]ResultMapMeta, owner string, discriminator ResultDiscriminatorMeta) {
	column := strings.TrimSpace(discriminator.Column)
	if column == "" && len(discriminator.Cases) > 0 {
		v.add("resultMap", owner, "resultMap %s discriminator column is required", owner)
	}
	if column != "" && len(discriminator.Cases) == 0 {
		v.add("resultMap", owner, "resultMap %s discriminator cases are required", owner)
	}
	v.validateTypeHandler(discriminator.TypeHandler, "discriminator", owner, discriminator.Column)
	for _, item := range discriminator.Cases {
		if strings.TrimSpace(item.ResultMap) != "" {
			v.validateResultMapReference(namespace, resultMaps, item.ResultMap, "discriminator", owner)
		}
		for _, field := range item.Fields {
			v.validateTypeHandler(field.TypeHandler, "discriminator field", owner, field.Property)
		}
		v.validateAssociations(namespace, owner, item.Associations)
		v.validateCollections(namespace, owner, item.Collections)
	}
}

func (v *registryValidator) validateAssociations(namespace string, owner string, associations []ResultAssociationMeta) {
	for _, association := range associations {
		v.validateNestedSelect(namespace, owner, "association", association.Property, association.Select, association.Column)
		for _, field := range association.Fields {
			v.validateTypeHandler(field.TypeHandler, "association field", owner, field.Property)
		}
		v.validateAssociations(namespace, owner, association.Associations)
		v.validateCollections(namespace, owner, association.Collections)
	}
}

func (v *registryValidator) validateCollections(namespace string, owner string, collections []ResultCollectionMeta) {
	for _, collection := range collections {
		v.validateNestedSelect(namespace, owner, "collection", collection.Property, collection.Select, collection.Column)
		for _, field := range collection.Fields {
			v.validateTypeHandler(field.TypeHandler, "collection field", owner, field.Property)
		}
		v.validateAssociations(namespace, owner, collection.Associations)
		v.validateCollections(namespace, owner, collection.Collections)
	}
}

func (v *registryValidator) validateNestedSelect(namespace string, owner string, kind string, property string, selectName string, column string) {
	selectName = strings.TrimSpace(selectName)
	if selectName == "" {
		return
	}
	if _, _, err := parseNestedSelectCompositeColumn(column); err != nil {
		v.addCause(kind, owner, fmt.Sprintf("%s %s nested select column is invalid", kind, property), err)
	}
	if v.statementExists(namespace, selectName) {
		return
	}
	v.add(kind, owner, "%s %s nested select %q is not registered", kind, property, selectName)
}

func (v *registryValidator) validateSelectKey(statementName string, statement StatementMeta) {
	if statement.UseGeneratedKeys && strings.TrimSpace(statement.KeyProperty) == "" {
		v.add("statement", statementName, "statement %s useGeneratedKeys requires keyProperty", statementName)
	}
	if !statement.SelectKey.Enabled {
		return
	}
	selectKey := statement.SelectKey
	if strings.TrimSpace(selectKey.KeyProperty) == "" && strings.TrimSpace(statement.KeyProperty) == "" {
		v.add("selectKey", statementName, "statement %s selectKey requires keyProperty", statementName)
	}
	hasSQL := strings.TrimSpace(selectKey.SQL) != ""
	hasDynamicSQL := len(selectKey.DynamicSQL) > 0
	switch {
	case !hasSQL && !hasDynamicSQL:
		v.add("selectKey", statementName, "statement %s selectKey requires SQL or DynamicSQL", statementName)
	case hasSQL && hasDynamicSQL:
		v.add("selectKey", statementName, "statement %s selectKey must not declare both SQL and DynamicSQL", statementName)
	}
	if !validSelectKeyOrder(selectKey.Order) {
		v.add("selectKey", statementName, "statement %s selectKey order %q is invalid", statementName, selectKey.Order)
	}
}

func (v *registryValidator) validateResultMapReference(namespace string, resultMaps map[string]ResultMapMeta, id string, resource string, owner string) {
	normalized := normalizeRuntimeResultMapID(namespace, id)
	if normalized == "" {
		v.add(resource, owner, "%s %s references empty resultMap", resource, owner)
		return
	}
	if _, ok := resultMaps[normalized]; !ok {
		v.add(resource, owner, "%s %s references unknown resultMap %q", resource, owner, id)
	}
}

func (v *registryValidator) validateResultMapReferenceCycles(mapper MapperMeta, resultMaps map[string]ResultMapMeta) {
	state := make(map[string]int, len(resultMaps))
	reported := make(map[string]struct{})
	var visit func(string, []string)
	visit = func(id string, stack []string) {
		switch state[id] {
		case 1:
			cycle := resultMapCycle(stack, id)
			if _, ok := reported[cycle]; !ok {
				reported[cycle] = struct{}{}
				v.add("resultMap", resultMapQualifiedName(mapper.Namespace, id), "resultMap reference cycle detected: %s", cycle)
			}
			return
		case 2:
			return
		}
		resultMap, ok := resultMaps[id]
		if !ok {
			return
		}
		state[id] = 1
		nextStack := append(stack, id)
		for _, ref := range resultMapReferences(mapper.Namespace, resultMap) {
			if _, ok := resultMaps[ref]; ok {
				visit(ref, nextStack)
			}
		}
		state[id] = 2
	}
	for _, id := range sortedValidationKeys(resultMaps) {
		visit(id, nil)
	}
}

func (v *registryValidator) validateCacheRef(namespace string) {
	seen := make(map[string]struct{})
	current := strings.TrimSpace(namespace)
	for current != "" {
		if _, ok := seen[current]; ok {
			v.add("cache-ref", namespace, "cache-ref cycle detected at namespace %s", current)
			return
		}
		seen[current] = struct{}{}
		ref := strings.TrimSpace(v.snapshot.cacheRefs[current])
		if ref == "" {
			if _, ok := v.snapshot.caches[current]; ok {
				return
			}
			v.add("cache-ref", namespace, "cache-ref for namespace %s resolves to %s without registered cache", namespace, current)
			return
		}
		current = ref
	}
	v.add("cache-ref", namespace, "cache-ref for namespace %s resolves to empty namespace", namespace)
}

func (v *registryValidator) validateTypeHandler(handlerName string, resource string, owner string, field string) {
	handlerName = strings.TrimSpace(handlerName)
	if handlerName == "" {
		return
	}
	if _, ok := v.snapshot.handlers[handlerName]; ok {
		return
	}
	if strings.TrimSpace(field) == "" {
		v.add("type-handler", owner, "%s %s references unknown type-handler %q", resource, owner, handlerName)
		return
	}
	v.add("type-handler", owner, "%s %s field %s references unknown type-handler %q", resource, owner, field, handlerName)
}

func (v *registryValidator) statementExists(namespace string, statement string) bool {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return false
	}
	if _, ok := v.snapshot.statements[statement]; ok {
		return true
	}
	_, ok := v.snapshot.statements[strings.TrimSpace(namespace)+"."+statement]
	return ok
}

func (v *registryValidator) add(resource string, name string, format string, args ...any) {
	v.errs = append(v.errs, registryErrorf(resource, name, format, args...))
}

func (v *registryValidator) addCause(resource string, name string, message string, err error) {
	v.errs = append(v.errs, &RegistryError{
		Resource: resource,
		Name:     name,
		Message:  message,
		Err:      err,
	})
}

func statementHasSQLSource(statement StatementMeta) bool {
	return strings.TrimSpace(statement.SQL) != "" || len(statement.DynamicSQL) > 0 || strings.TrimSpace(statement.Provider) != ""
}

func validStatementCommand(command StatementCommand) bool {
	switch command {
	case StatementCommandSelect, StatementCommandInsert, StatementCommandUpdate, StatementCommandDelete, StatementCommandCall:
		return true
	default:
		return false
	}
}

func validSelectKeyOrder(order SelectKeyOrder) bool {
	switch strings.ToUpper(strings.TrimSpace(string(order))) {
	case "", string(SelectKeyOrderBefore), string(SelectKeyOrderAfter):
		return true
	default:
		return false
	}
}

func validationStatementName(namespace string, statement StatementMeta) string {
	if fullName := strings.TrimSpace(statement.FullName); fullName != "" {
		return fullName
	}
	if id := strings.TrimSpace(statement.ID); id != "" && strings.TrimSpace(namespace) != "" {
		return strings.TrimSpace(namespace) + "." + id
	}
	if id := strings.TrimSpace(statement.ID); id != "" {
		return id
	}
	return strings.TrimSpace(namespace)
}

func resultMapReferences(namespace string, resultMap ResultMapMeta) []string {
	refs := make([]string, 0, 1+len(resultMap.Discriminator.Cases))
	if ref := normalizeRuntimeResultMapID(namespace, resultMap.Extends); ref != "" {
		refs = append(refs, ref)
	}
	for _, item := range resultMap.Discriminator.Cases {
		if ref := normalizeRuntimeResultMapID(namespace, item.ResultMap); ref != "" {
			refs = append(refs, ref)
		}
	}
	return refs
}

func resultMapCycle(stack []string, id string) string {
	start := 0
	for index, item := range stack {
		if item == id {
			start = index
			break
		}
	}
	cycle := append(append([]string(nil), stack[start:]...), id)
	return strings.Join(cycle, " -> ")
}

func resultMapQualifiedName(namespace string, id string) string {
	namespace = strings.TrimSpace(namespace)
	id = strings.TrimSpace(id)
	if namespace == "" || strings.HasPrefix(id, namespace+".") {
		return id
	}
	if id == "" {
		return namespace
	}
	return namespace + "." + id
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func sortedValidationKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
