package orm

import (
	"context"
	"fmt"
	"strings"
)

// SQLSource 表示 Provider 在运行期返回的 SQL 来源。
type SQLSource struct {
	SQL        string
	DynamicSQL []DynamicSQLNode
	Args       NamedArgs
	CacheKey   string
}

// SQLProvider 按 Statement 和入参在运行期生成 SQL。
type SQLProvider func(ctx context.Context, statement StatementMeta, args NamedArgs) (SQLSource, error)

// SQLProviderDescriptor 描述一个可注册的 SQL Provider。
type SQLProviderDescriptor struct {
	Name       string
	Provider   SQLProvider
	Statements []string
	Commands   []StatementCommand
}

// SQLProviderOption 配置 SQL Provider 描述。
type SQLProviderOption func(*SQLProviderDescriptor)

// NewSQLProviderDescriptor 创建 SQL Provider 描述。
func NewSQLProviderDescriptor(name string, provider SQLProvider, options ...SQLProviderOption) SQLProviderDescriptor {
	descriptor := SQLProviderDescriptor{
		Name:     name,
		Provider: provider,
	}
	for _, option := range options {
		if option != nil {
			option(&descriptor)
		}
	}
	return descriptor
}

// WithSQLProviderStatements 限定 Provider 允许服务的 Statement 完整名称。
func WithSQLProviderStatements(statements ...string) SQLProviderOption {
	return func(descriptor *SQLProviderDescriptor) {
		if descriptor == nil {
			return
		}
		descriptor.Statements = append(descriptor.Statements, statements...)
	}
}

// WithSQLProviderCommands 限定 Provider 允许服务的 Statement 命令类型。
func WithSQLProviderCommands(commands ...StatementCommand) SQLProviderOption {
	return func(descriptor *SQLProviderDescriptor) {
		if descriptor == nil {
			return
		}
		descriptor.Commands = append(descriptor.Commands, commands...)
	}
}

// ValidateStatement 校验 Provider 是否允许当前 Statement 使用。
func (d SQLProviderDescriptor) ValidateStatement(statement StatementMeta) error {
	name := strings.TrimSpace(d.Name)
	statementName := strings.TrimSpace(statement.FullName)
	if len(d.Statements) > 0 && !containsSQLProviderStatement(d.Statements, statementName) {
		return &BindingError{
			Statement: statement.FullName,
			Operation: "validate SQL provider",
			Message: fmt.Sprintf(
				"SQL provider %q is not allowed for statement %s",
				name,
				statementName,
			),
		}
	}
	if len(d.Commands) > 0 && !containsSQLProviderCommand(d.Commands, statement.Command) {
		return &BindingError{
			Statement: statement.FullName,
			Operation: "validate SQL provider",
			Message: fmt.Sprintf(
				"SQL provider %q does not allow command %s for statement %s",
				name,
				statement.Command,
				statementName,
			),
		}
	}
	return nil
}

func normalizeSQLProviderDescriptor(descriptor SQLProviderDescriptor) (SQLProviderDescriptor, error) {
	descriptor.Name = strings.TrimSpace(descriptor.Name)
	if descriptor.Name == "" {
		return SQLProviderDescriptor{}, registryErrorf("SQL provider", "", "SQL provider name is required")
	}
	if descriptor.Provider == nil {
		return SQLProviderDescriptor{}, registryErrorf("SQL provider", descriptor.Name, "SQL provider %q is nil", descriptor.Name)
	}
	statements, err := compactSQLProviderStatements(descriptor.Statements)
	if err != nil {
		return SQLProviderDescriptor{}, err
	}
	commands, err := compactSQLProviderCommands(descriptor.Commands)
	if err != nil {
		return SQLProviderDescriptor{}, err
	}
	descriptor.Statements = statements
	descriptor.Commands = commands
	return descriptor, nil
}

func copySQLProviderDescriptor(descriptor SQLProviderDescriptor) SQLProviderDescriptor {
	descriptor.Statements = append([]string(nil), descriptor.Statements...)
	descriptor.Commands = append([]StatementCommand(nil), descriptor.Commands...)
	return descriptor
}

func compactSQLProviderStatements(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, registryErrorf("SQL provider", "", "SQL provider statement is required")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func compactSQLProviderCommands(values []StatementCommand) ([]StatementCommand, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make([]StatementCommand, 0, len(values))
	seen := make(map[StatementCommand]struct{}, len(values))
	for _, value := range values {
		value = StatementCommand(strings.TrimSpace(string(value)))
		if value == "" {
			return nil, registryErrorf("SQL provider", "", "SQL provider command is required")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out, nil
}

func containsSQLProviderStatement(values []string, statement string) bool {
	statement = strings.TrimSpace(statement)
	for _, value := range values {
		if value == statement {
			return true
		}
	}
	return false
}

func containsSQLProviderCommand(values []StatementCommand, command StatementCommand) bool {
	for _, value := range values {
		if value == command {
			return true
		}
	}
	return false
}
