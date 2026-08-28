package account

import (
	"context"
	"fmt"
	"strings"

	orm "goark.dev/orm"
)

const (
	// ActiveEmailsProviderName 是账号邮箱查询 Provider 的稳定名称。
	ActiveEmailsProviderName = "examples.production.account.UserSQL.ActiveEmails"

	maxActiveEmailLimit = 1000
)

// RegisterSQLProviders 注册账号模块的显式 SQL Provider。
func RegisterSQLProviders(registry *orm.Registry) error {
	if registry == nil {
		return fmt.Errorf("goark-orm example: registry is nil")
	}
	return registry.RegisterSQLProviderDescriptor(orm.NewSQLProviderDescriptor(
		ActiveEmailsProviderName,
		activeEmailsProvider,
		orm.WithSQLProviderCommands(orm.StatementCommandSelect),
		orm.WithSQLProviderStatements(UserMapperNamespace+".ActiveEmails"),
	))
}

func activeEmailsProvider(ctx context.Context, statement orm.StatementMeta, args orm.NamedArgs) (orm.SQLSource, error) {
	if ctx == nil {
		return orm.SQLSource{}, fmt.Errorf("goark-orm example: context is nil")
	}
	if err := ctx.Err(); err != nil {
		return orm.SQLSource{}, err
	}
	tenantID, err := requiredStringArg(args, "tenantID")
	if err != nil {
		return orm.SQLSource{}, err
	}
	limit, err := boundedIntArg(args, "limit", 100, maxActiveEmailLimit)
	if err != nil {
		return orm.SQLSource{}, err
	}
	return orm.NewSelectSQLBuilder().
		Select("email").
		From("sys_user").
		WhereEq("tenant_id", tenantID).
		WhereEq("status", string(UserStatusActive)).
		WhereEq("deleted", false).
		OrderByAsc("email").
		Limit(limit).
		CacheKey("tenant:" + tenantID + ":" + statement.ID).
		Build()
}

func requiredStringArg(args orm.NamedArgs, name string) (string, error) {
	value, ok := args[name]
	if !ok {
		return "", fmt.Errorf("goark-orm example: argument %s is required", name)
	}
	text, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("goark-orm example: argument %s must be string", name)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("goark-orm example: argument %s is required", name)
	}
	return text, nil
}

func boundedIntArg(args orm.NamedArgs, name string, fallback int, max int) (int, error) {
	value, ok := args[name]
	if !ok || value == nil {
		return fallback, nil
	}
	var out int
	switch typed := value.(type) {
	case int:
		out = typed
	case int8:
		out = int(typed)
	case int16:
		out = int(typed)
	case int32:
		out = int(typed)
	case int64:
		if typed > int64(max) {
			return max, nil
		}
		out = int(typed)
	default:
		return 0, fmt.Errorf("goark-orm example: argument %s must be integer", name)
	}
	if out <= 0 {
		return fallback, nil
	}
	if out > max {
		return max, nil
	}
	return out, nil
}
