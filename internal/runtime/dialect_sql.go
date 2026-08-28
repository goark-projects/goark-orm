package runtime

import (
	"fmt"
	"strings"
)

// UpsertSpec 描述一次方言 UPSERT SQL 构造请求。
type UpsertSpec struct {
	Table            string
	InsertColumns    []string
	ConflictColumns  []string
	UpdateColumns    []string
	ReturningColumns []string
	Values           NamedArgs
	CacheKey         string
}

// RowLockOptions 描述 SELECT 行锁尾句选项。
type RowLockOptions struct {
	SkipLocked bool
	NoWait     bool
}

// GeneratedKeyPlan 描述指定方言的生成主键回读策略。
type GeneratedKeyPlan struct {
	Style            DialectGeneratedKeyStyle
	KeyColumn        string
	SQLClause        string
	UsesLastInsertID bool
}

// BuildUpsertSQL 根据方言构造参数化 UPSERT SQLSource。
func BuildUpsertSQL(dialect Dialect, spec UpsertSpec) (SQLSource, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	capabilities := DialectCapabilitiesOf(dialect)
	switch capabilities.Upsert {
	case DialectUpsertOnConflict:
		return buildOnConflictUpsertSQL(spec)
	case DialectUpsertOnDuplicateKey:
		return buildOnDuplicateKeyUpsertSQL(spec)
	case DialectUpsertMerge:
		return buildMergeUpsertSQL(dialect, spec)
	default:
		return SQLSource{}, configurationErrorf("upsert is not supported by dialect %q", dialect.Name())
	}
}

// RowLockClause 返回指定方言可追加到 SELECT 语句末尾的行锁子句。
func RowLockClause(dialect Dialect, options RowLockOptions) (string, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	capabilities := DialectCapabilitiesOf(dialect)
	switch capabilities.RowLock {
	case DialectRowLockForUpdate:
		parts := []string{"FOR UPDATE"}
		if options.SkipLocked {
			if !capabilities.SkipLocked {
				return "", configurationErrorf("dialect %q does not support SKIP LOCKED", dialect.Name())
			}
			parts = append(parts, "SKIP LOCKED")
		}
		if options.NoWait {
			if !capabilities.NoWait {
				return "", configurationErrorf("dialect %q does not support NOWAIT", dialect.Name())
			}
			parts = append(parts, "NOWAIT")
		}
		return strings.Join(parts, " "), nil
	case DialectRowLockHints:
		if options.SkipLocked || options.NoWait {
			return "", configurationErrorf("dialect %q row lock hints do not support requested wait option", dialect.Name())
		}
		return "WITH (UPDLOCK, ROWLOCK)", nil
	default:
		return "", configurationErrorf("row lock is not supported by dialect %q", dialect.Name())
	}
}

// NewGeneratedKeyPlan 返回指定方言的生成主键回读计划。
func NewGeneratedKeyPlan(dialect Dialect, keyColumn string) (GeneratedKeyPlan, error) {
	if dialect == nil {
		dialect = NewQuestionDialect()
	}
	capabilities := DialectCapabilitiesOf(dialect)
	keyColumn = strings.TrimSpace(keyColumn)
	plan := GeneratedKeyPlan{
		Style:     capabilities.GeneratedKey,
		KeyColumn: keyColumn,
	}
	switch capabilities.GeneratedKey {
	case DialectGeneratedKeyNone:
		return plan, nil
	case DialectGeneratedKeyLastInsertID:
		plan.UsesLastInsertID = true
		return plan, nil
	case DialectGeneratedKeyReturning:
		column, err := generatedKeyColumnSQL(dialect, keyColumn)
		if err != nil {
			return GeneratedKeyPlan{}, err
		}
		plan.SQLClause = "RETURNING " + column
		return plan, nil
	case DialectGeneratedKeyOutput:
		column, err := generatedKeyColumnSQL(dialect, keyColumn)
		if err != nil {
			return GeneratedKeyPlan{}, err
		}
		plan.SQLClause = "OUTPUT inserted." + column
		return plan, nil
	case DialectGeneratedKeyReturningInto:
		column, err := generatedKeyColumnSQL(dialect, keyColumn)
		if err != nil {
			return GeneratedKeyPlan{}, err
		}
		plan.SQLClause = "RETURNING " + column + " INTO ?"
		return plan, nil
	default:
		return GeneratedKeyPlan{}, configurationErrorf("generated key style %q is not supported", capabilities.GeneratedKey)
	}
}

func buildOnConflictUpsertSQL(spec UpsertSpec) (SQLSource, error) {
	if len(spec.ConflictColumns) == 0 {
		return SQLSource{}, configurationErrorf("on conflict upsert requires conflict columns")
	}
	state := newSQLBuilderState()
	insertSQL, err := buildUpsertInsertSQL(state, spec)
	if err != nil {
		return SQLSource{}, err
	}
	conflictColumns, err := state.identifierList(spec.ConflictColumns, "")
	if err != nil {
		return SQLSource{}, err
	}
	var builder strings.Builder
	builder.WriteString(insertSQL)
	builder.WriteString(" ON CONFLICT (")
	builder.WriteString(strings.Join(conflictColumns, ", "))
	builder.WriteString(")")
	if len(spec.UpdateColumns) == 0 {
		builder.WriteString(" DO NOTHING")
	} else {
		sets, err := state.excludedAssignments(spec.UpdateColumns)
		if err != nil {
			return SQLSource{}, err
		}
		builder.WriteString(" DO UPDATE SET ")
		builder.WriteString(strings.Join(sets, ", "))
	}
	returning, err := state.identifierClause("RETURNING", spec.ReturningColumns)
	if err != nil {
		return SQLSource{}, err
	}
	if returning != "" {
		builder.WriteByte(' ')
		builder.WriteString(returning)
	}
	return state.source(builder.String(), spec.CacheKey), nil
}

func buildOnDuplicateKeyUpsertSQL(spec UpsertSpec) (SQLSource, error) {
	if len(spec.ReturningColumns) > 0 {
		return SQLSource{}, configurationErrorf("on duplicate key upsert does not support returning columns")
	}
	if len(spec.UpdateColumns) == 0 {
		return SQLSource{}, configurationErrorf("on duplicate key upsert requires update columns")
	}
	state := newSQLBuilderState()
	insertSQL, err := buildUpsertInsertSQL(state, spec)
	if err != nil {
		return SQLSource{}, err
	}
	sets, err := state.valuesAssignments(spec.UpdateColumns)
	if err != nil {
		return SQLSource{}, err
	}
	return state.source(insertSQL+" ON DUPLICATE KEY UPDATE "+strings.Join(sets, ", "), spec.CacheKey), nil
}

func buildUpsertInsertSQL(state *sqlBuilderState, spec UpsertSpec) (string, error) {
	if state == nil {
		return "", configurationErrorf("upsert SQL builder state is nil")
	}
	if len(spec.InsertColumns) == 0 {
		return "", configurationErrorf("upsert requires insert columns")
	}
	table, err := state.identifier(spec.Table)
	if err != nil {
		return "", err
	}
	columns := make([]string, 0, len(spec.InsertColumns))
	values := make([]string, 0, len(spec.InsertColumns))
	for _, columnName := range spec.InsertColumns {
		column, err := state.identifier(columnName)
		if err != nil {
			return "", err
		}
		value, ok := spec.Values[strings.TrimSpace(columnName)]
		if !ok {
			return "", &BindingError{
				Operation: "build upsert",
				Parameter: strings.TrimSpace(columnName),
				Message:   fmt.Sprintf("upsert value for column %q is missing", strings.TrimSpace(columnName)),
			}
		}
		columns = append(columns, column)
		values = append(values, state.value(value))
	}
	return fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(columns, ", "), strings.Join(values, ", ")), nil
}

func (s *sqlBuilderState) excludedAssignments(columns []string) ([]string, error) {
	out := make([]string, 0, len(columns))
	for _, columnName := range columns {
		column, err := s.identifier(columnName)
		if err != nil {
			return nil, err
		}
		excluded, err := s.identifier(columnName)
		if err != nil {
			return nil, err
		}
		out = append(out, column+" = EXCLUDED."+excluded)
	}
	return out, nil
}

func (s *sqlBuilderState) valuesAssignments(columns []string) ([]string, error) {
	out := make([]string, 0, len(columns))
	for _, columnName := range columns {
		column, err := s.identifier(columnName)
		if err != nil {
			return nil, err
		}
		valueColumn, err := s.identifier(columnName)
		if err != nil {
			return nil, err
		}
		out = append(out, column+" = VALUES("+valueColumn+")")
	}
	return out, nil
}

func generatedKeyColumnSQL(dialect Dialect, keyColumn string) (string, error) {
	if strings.TrimSpace(keyColumn) == "" {
		return "", configurationErrorf("generated key column is required")
	}
	return quoteIdentifierPath(dialect, keyColumn)
}
