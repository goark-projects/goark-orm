package orm

import (
	"strconv"
	"strings"
)

type statementParameterPlanKey struct {
	FullName   string
	SQL        string
	Parameters string
	DynamicSQL string
}

// statementParameterSet 返回当前 Statement 实际使用的命名参数集合。
func (s *SQLSession) statementParameterSet(statement StatementMeta) map[string]struct{} {
	if s == nil {
		return buildStatementParameterSet(statement)
	}
	key := statementParameterPlanKey{
		FullName:   strings.TrimSpace(statement.FullName),
		SQL:        statement.SQL,
		Parameters: strings.Join(statement.Parameters, "\x00"),
		DynamicSQL: dynamicSQLPlanKey(statement.DynamicSQL),
	}
	if cached, ok := s.statementParameterPlans.Load(key); ok {
		return cached.(map[string]struct{})
	}
	parameters := buildStatementParameterSet(statement)
	actual, _ := s.statementParameterPlans.LoadOrStore(key, parameters)
	return actual.(map[string]struct{})
}

func dynamicSQLPlanKey(nodes []DynamicSQLNode) string {
	if len(nodes) == 0 {
		return ""
	}
	var builder strings.Builder
	writeDynamicSQLPlanKey(&builder, nodes)
	return builder.String()
}

func writeDynamicSQLPlanKey(builder *strings.Builder, nodes []DynamicSQLNode) {
	for _, node := range nodes {
		builder.WriteString(string(node.Kind))
		builder.WriteByte(':')
		builder.WriteString(strconv.Itoa(len(node.Children)))
		builder.WriteByte(':')
		builder.WriteString(node.Text)
		builder.WriteByte(':')
		builder.WriteString(node.Value)
		builder.WriteByte(';')
		writeDynamicSQLPlanKey(builder, node.Children)
	}
}
