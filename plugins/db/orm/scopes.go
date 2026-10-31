package orm

import (
	"context"
	"fmt"
	"strings"
	"time"
)

//
// ========================== Core ==========================
//

type Scope func(context.Context, *Builder) *Builder

type ScopeType int

const (
	ScopeQuery ScopeType = iota
	ScopeRouting
	ScopeMeta
)

type Scoped struct {
	Name string
	Type ScopeType
	Fn   Scope
}

//
// ========================== Builder ==========================
//

type Builder struct {
	ctx context.Context

	table string

	where []string
	args  []interface{}

	orderBy []string
	limit   *int
	offset  *int

	scopes  []Scoped
	applied []string
}

func NewBuilder(ctx context.Context, table string) *Builder {
	if ctx == nil {
		ctx = context.Background()
	}
	return &Builder{
		ctx:   ctx,
		table: table,
	}
}

//
// ========================== Scope Registration ==========================
//

func (b *Builder) Scope(fn Scope) *Builder {
	b.scopes = append(b.scopes, Scoped{Fn: fn, Type: ScopeQuery})
	return b
}

func (b *Builder) Routing(fn Scope) *Builder {
	b.scopes = append(b.scopes, Scoped{Fn: fn, Type: ScopeRouting})
	return b
}

func (b *Builder) Meta(fn Scope) *Builder {
	b.scopes = append(b.scopes, Scoped{Fn: fn, Type: ScopeMeta})
	return b
}

func (b *Builder) Add(s Scoped) *Builder {
	b.scopes = append(b.scopes, s)
	return b
}

func Named(name string, t ScopeType, fn Scope) Scoped {
	return Scoped{Name: name, Type: t, Fn: fn}
}

//
// ========================== Apply ==========================
//

func (b *Builder) ApplyScopes() *Builder {
	b.applyByType(b.ctx, ScopeRouting)
	b.applyByType(b.ctx, ScopeQuery)
	b.applyByType(b.ctx, ScopeMeta)
	return b
}

func (b *Builder) applyByType(ctx context.Context, t ScopeType) {
	for _, s := range b.scopes {
		if s.Type != t {
			continue
		}

		newB := s.Fn(ctx, b)
		if newB != nil {
			*b = *newB // ✅ 修复核心
		}

		if s.Name != "" {
			b.applied = append(b.applied, s.Name)
		}
	}
}

//
// ========================== Builder Ops ==========================
//

func (b *Builder) Table(name string) *Builder {
	b.table = name
	return b
}

func (b *Builder) Where(cond string, args ...interface{}) *Builder {
	b.where = append(b.where, cond)
	b.args = append(b.args, args...)
	return b
}

func (b *Builder) OrderBy(expr string) *Builder {
	b.orderBy = append(b.orderBy, expr)
	return b
}

func (b *Builder) Limit(n int) *Builder {
	b.limit = &n
	return b
}

func (b *Builder) Offset(n int) *Builder {
	b.offset = &n
	return b
}

func (b *Builder) AppliedScopes() []string {
	return b.applied
}

//
// ========================== Build SQL ==========================
//

func (b *Builder) Build() (string, []interface{}) {
	var sb strings.Builder

	sb.WriteString("SELECT * FROM ")
	sb.WriteString(b.table)

	if len(b.where) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.where, " AND "))
	}

	if len(b.orderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(b.orderBy, ", "))
	}

	if b.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *b.limit))
	}

	if b.offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", *b.offset))
	}

	return sb.String(), b.args
}

//
// ========================== Scopes ==========================
//

func Active(column string) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		return b.Where(column+" = ?", "active")
	}
}

func NotDeleted() Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		return b.Where("deleted_at IS NULL")
	}
}

func Paginate(page, size int) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		if page < 1 {
			page = 1
		}
		if size <= 0 {
			size = 10
		}
		offset := (page - 1) * size
		return b.Limit(size).Offset(offset)
	}
}

func OrderByDesc(col string) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		return b.OrderBy(col + " DESC")
	}
}

func CreatedAfter(t time.Time) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		return b.Where("created_at >= ?", t)
	}
}

//
// ========================== Routing ==========================
//

func HashTable(base string, shardKey int64, count int) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		idx := shardKey % int64(count)
		return b.Table(fmt.Sprintf("%s_%d", base, idx))
	}
}

func ShardByUser(userID int64) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		return b.Where("user_id = ?", userID)
	}
}

//
// ========================== Meta ==========================
//

func Trace(name string) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		// 可接入 logger / tracing system
		return b
	}
}

//
// ========================== Helpers ==========================
//

func Compose(scopes ...Scope) Scope {
	return func(ctx context.Context, b *Builder) *Builder {
		for _, s := range scopes {
			nb := s(ctx, b)
			if nb != nil {
				b = nb
			}
		}
		return b
	}
}

func If(cond bool, s Scope) Scope {
	if !cond {
		return func(ctx context.Context, b *Builder) *Builder { return b }
	}
	return s
}

func IfNotZero[T comparable](v T, fn func(T) Scope) Scope {
	var zero T
	if v == zero {
		return func(ctx context.Context, b *Builder) *Builder { return b }
	}
	return fn(v)
}
