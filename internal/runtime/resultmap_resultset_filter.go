package runtime

import "strings"

func resultMapHasResultSetMappings(resultMap ResultMapMeta) bool {
	if resultObjectsHaveResultSetMappings(resultMap.Associations, resultMap.Collections) {
		return true
	}
	for _, item := range resultMap.Discriminator.Cases {
		if resultObjectsHaveResultSetMappings(item.Associations, item.Collections) {
			return true
		}
	}
	return false
}

func resultObjectsHaveResultSetMappings(associations []ResultAssociationMeta, collections []ResultCollectionMeta) bool {
	for _, association := range associations {
		if strings.TrimSpace(association.ResultSet) != "" ||
			resultObjectsHaveResultSetMappings(association.Associations, association.Collections) {
			return true
		}
	}
	for _, collection := range collections {
		if strings.TrimSpace(collection.ResultSet) != "" ||
			resultObjectsHaveResultSetMappings(collection.Associations, collection.Collections) {
			return true
		}
	}
	return false
}

func resultMapWithoutResultSetMappings(resultMap ResultMapMeta) ResultMapMeta {
	resultMap.Associations = associationsWithoutResultSetMappings(resultMap.Associations)
	resultMap.Collections = collectionsWithoutResultSetMappings(resultMap.Collections)
	return resultMap
}

func associationsWithoutResultSetMappings(items []ResultAssociationMeta) []ResultAssociationMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]ResultAssociationMeta, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ResultSet) != "" {
			continue
		}
		item.Associations = associationsWithoutResultSetMappings(item.Associations)
		item.Collections = collectionsWithoutResultSetMappings(item.Collections)
		out = append(out, item)
	}
	return out
}

func collectionsWithoutResultSetMappings(items []ResultCollectionMeta) []ResultCollectionMeta {
	if len(items) == 0 {
		return nil
	}
	out := make([]ResultCollectionMeta, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ResultSet) != "" {
			continue
		}
		item.Associations = associationsWithoutResultSetMappings(item.Associations)
		item.Collections = collectionsWithoutResultSetMappings(item.Collections)
		out = append(out, item)
	}
	return out
}
