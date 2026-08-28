package runtime

import "strings"

func (v *registryValidator) validateStatementResultSetMappings(namespace string, resultMaps map[string]ResultMapMeta, statement StatementMeta, owner string) {
	resultMap, ok := resultMaps[normalizeRuntimeResultMapID(namespace, statement.ResultMap)]
	if !ok {
		return
	}
	declared := statementResultSetNames(statement.ResultSets)
	v.validateResultSetAssociations(owner, declared, resultMap.Associations, false)
	v.validateResultSetCollections(owner, declared, resultMap.Collections, false)
	for _, item := range resultMap.Discriminator.Cases {
		v.validateResultSetAssociations(owner, declared, item.Associations, false)
		v.validateResultSetCollections(owner, declared, item.Collections, false)
	}
}

func statementResultSetNames(resultSets []ResultSetMeta) map[string]struct{} {
	out := make(map[string]struct{}, len(resultSets))
	for _, resultSet := range resultSets {
		if name := strings.TrimSpace(resultSet.Name); name != "" {
			out[name] = struct{}{}
		}
	}
	return out
}

func (v *registryValidator) validateResultSetAssociations(owner string, declared map[string]struct{}, associations []ResultAssociationMeta, nested bool) {
	for _, association := range associations {
		resultSet := strings.TrimSpace(association.ResultSet)
		if resultSet != "" {
			v.validateResultSetObject(owner, declared, "association", association.Property, resultSet, association.Column, association.ForeignColumn, nested)
		}
		v.validateResultSetAssociations(owner, declared, association.Associations, nested || resultSet != "")
		v.validateResultSetCollections(owner, declared, association.Collections, nested || resultSet != "")
	}
}

func (v *registryValidator) validateResultSetCollections(owner string, declared map[string]struct{}, collections []ResultCollectionMeta, nested bool) {
	for _, collection := range collections {
		resultSet := strings.TrimSpace(collection.ResultSet)
		if resultSet != "" {
			v.validateResultSetObject(owner, declared, "collection", collection.Property, resultSet, collection.Column, collection.ForeignColumn, nested)
		}
		v.validateResultSetAssociations(owner, declared, collection.Associations, nested || resultSet != "")
		v.validateResultSetCollections(owner, declared, collection.Collections, nested || resultSet != "")
	}
}

func (v *registryValidator) validateResultSetObject(owner string, declared map[string]struct{}, kind string, property string, resultSet string, column string, foreignColumn string, nested bool) {
	if nested {
		v.add(kind, owner, "%s %s resultSet %q must be declared at root resultMap level", kind, property, resultSet)
	}
	if _, ok := declared[resultSet]; !ok {
		v.add(kind, owner, "%s %s references undeclared resultSet %q", kind, property, resultSet)
	}
	if _, _, err := resultSetJoinColumns(column, foreignColumn); err != nil {
		v.addCause(kind, owner, kind+" "+property+" resultSet foreignColumn is invalid", err)
	}
}
