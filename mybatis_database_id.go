package orm

import "strings"

const (
	// DatabaseIDProviderVendor 表示按数据库厂商名称映射 databaseId。
	DatabaseIDProviderVendor = "vendor"
)

// DatabaseIDProvider 描述 Go 原生 databaseId 推导规则。
type DatabaseIDProvider struct {
	Type       string
	Properties ConfigProperties
	DefaultID  string
}

func (p DatabaseIDProvider) resolve(environment MyBatisEnvironment) (string, error) {
	providerType := strings.ToLower(strings.TrimSpace(p.Type))
	if providerType == "" {
		return "", nil
	}
	if providerType != DatabaseIDProviderVendor {
		return "", configurationErrorf("databaseIdProvider type %q is invalid", p.Type)
	}
	if environment.Dialect == nil && environment.DbType == "" {
		return strings.TrimSpace(p.DefaultID), nil
	}
	candidates := databaseIDCandidates(environment)
	for _, candidate := range candidates {
		if mapped, ok := lookupDatabaseID(p.Properties, candidate); ok {
			return mapped, nil
		}
	}
	return strings.TrimSpace(p.DefaultID), nil
}

func (p DatabaseIDProvider) validate() error {
	providerType := strings.ToLower(strings.TrimSpace(p.Type))
	switch providerType {
	case "":
		return nil
	case DatabaseIDProviderVendor:
	default:
		return configurationErrorf("databaseIdProvider type %q is invalid", p.Type)
	}
	for key, value := range p.Properties {
		if strings.TrimSpace(key) == "" {
			return configurationErrorf("databaseIdProvider property name is required")
		}
		if strings.TrimSpace(value) == "" {
			return configurationErrorf("databaseIdProvider property %q value is required", key)
		}
	}
	return nil
}

func databaseIDCandidates(environment MyBatisEnvironment) []string {
	out := make([]string, 0, 4)
	if environment.Dialect != nil {
		out = append(out, environment.Dialect.Name())
	}
	if environment.DbType != "" {
		out = append(out, string(environment.DbType))
	}
	return out
}

func lookupDatabaseID(properties ConfigProperties, key string) (string, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "" {
		return "", false
	}
	for candidate, value := range properties {
		if strings.ToLower(strings.TrimSpace(candidate)) == key {
			return strings.TrimSpace(value), true
		}
	}
	return "", false
}
