package orm

import (
	"fmt"
	"strings"
)

type JoinOnBuilder struct {
	conditions []string
	args       []interface{}
	dialect    Dialect
}

func NewJoinOnBuilder(dialect Dialect) *JoinOnBuilder {
	if dialect == nil {
		dialect = NewDefaultDialect()
	}
	return &JoinOnBuilder{dialect: dialect}
}

func (jb *JoinOnBuilder) On(condition string, args ...interface{}) *JoinOnBuilder {
	jb.conditions = append(jb.conditions, condition)
	jb.args = append(jb.args, args...)
	return jb
}

func (jb *JoinOnBuilder) And(condition string, args ...interface{}) *JoinOnBuilder {
	if len(jb.conditions) > 0 {
		jb.conditions = append(jb.conditions, "AND "+condition)
	} else {
		jb.conditions = append(jb.conditions, condition)
	}
	jb.args = append(jb.args, args...)
	return jb
}

func (jb *JoinOnBuilder) Or(condition string, args ...interface{}) *JoinOnBuilder {
	if len(jb.conditions) > 0 {
		jb.conditions = append(jb.conditions, "OR "+condition)
	} else {
		jb.conditions = append(jb.conditions, condition)
	}
	jb.args = append(jb.args, args...)
	return jb
}

func (jb *JoinOnBuilder) Raw(condition string, args ...interface{}) *JoinOnBuilder {
	return jb.On(condition, args...)
}

func (jb *JoinOnBuilder) Eq(left, right string) *JoinOnBuilder {
	cond := fmt.Sprintf("%s = %s", jb.dialect.QuoteIdentifier(left), jb.dialect.QuoteIdentifier(right))
	return jb.On(cond)
}

func (jb *JoinOnBuilder) EqValue(column string, value interface{}) *JoinOnBuilder {
	cond := fmt.Sprintf("%s = ?", jb.dialect.QuoteIdentifier(column))
	return jb.On(cond, value)
}

func (jb *JoinOnBuilder) Build() (string, []interface{}) {
	if len(jb.conditions) == 0 {
		return "", nil
	}
	return strings.Join(jb.conditions, " "), jb.args
}

func (jb *JoinOnBuilder) IsEmpty() bool {
	return len(jb.conditions) == 0
}

func (jb *JoinOnBuilder) Clone() *JoinOnBuilder {
	c := &JoinOnBuilder{
		conditions: make([]string, len(jb.conditions)),
		args:       make([]interface{}, len(jb.args)),
		dialect:    jb.dialect,
	}
	copy(c.conditions, jb.conditions)
	copy(c.args, jb.args)
	return c
}
