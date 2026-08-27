package orm

import "context"

// EnumValuer 描述可显式暴露数据库入库值的枚举接口。
type EnumValuer interface {
	EnumValue() any
}

// EnumValuerContext 支持需要上下文的枚举值转换。
type EnumValuerContext interface {
	EnumValueContext(ctx context.Context) (any, error)
}

func databaseEnumValue(ctx context.Context, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if item, ok := value.(EnumValuerContext); ok {
		return item.EnumValueContext(ctx)
	}
	if item, ok := value.(EnumValuer); ok {
		return item.EnumValue(), nil
	}
	return value, nil
}
