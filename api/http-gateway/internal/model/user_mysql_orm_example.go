package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alldev-run/golang-gin-rpc/pkg/db/mysql"
	"github.com/alldev-run/golang-gin-rpc/pkg/db/orm"
)

const (
	userTableName      = "users"
	defaultListLimit   = 20
	maxListLimit       = 200
	defaultListOffset  = 0
)

type UserEntity struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	Email     string    `db:"email" json:"email"`
	Age       int       `db:"age" json:"age"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type UserRepository struct {
	db *mysql.Client
}

func NewUserRepository(db *mysql.Client) *UserRepository {
	return &UserRepository{db: db}
}

func (u *UserEntity) ValidateForWrite() error {
	if u == nil {
		return errors.New("user is nil")
	}
	if strings.TrimSpace(u.ID) == "" {
		return errors.New("user id is required")
	}
	if strings.TrimSpace(u.Name) == "" {
		return errors.New("user name is required")
	}
	if strings.TrimSpace(u.Email) == "" {
		return errors.New("user email is required")
	}
	if u.Age < 0 {
		return errors.New("user age cannot be negative")
	}
	return nil
}

func normalizeListArgs(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	if offset < 0 {
		offset = defaultListOffset
	}
	return limit, offset
}

func (r *UserRepository) ensureReady() error {
	if r == nil {
		return errors.New("user repository is nil")
	}
	if r.db == nil {
		return errors.New("mysql client is nil")
	}
	return nil
}

func NewMySQLClientFromConfig(cfg mysql.Config) (*mysql.Client, error) {
	return mysql.New(cfg)
}

func (r *UserRepository) Create(ctx context.Context, u *UserEntity) error {
	if err := r.ensureReady(); err != nil {
		return err
	}
	if err := u.ValidateForWrite(); err != nil {
		return err
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	_, err := orm.NewInsertBuilder(r.db, userTableName).Sets(map[string]interface{}{
		"id":         u.ID,
		"name":       u.Name,
		"email":      u.Email,
		"age":        u.Age,
		"created_at": u.CreatedAt,
	}).Exec(ctx)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*UserEntity, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("id is required")
	}

	rows, err := orm.NewSelectBuilder(r.db, userTableName).
		Columns("id", "name", "email", "age", "created_at").
		Eq("id", id).
		Limit(1).
		Query(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("user %s: %w", id, sql.ErrNoRows)
	}

	var u UserEntity
	if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*UserEntity, error) {
	if err := r.ensureReady(); err != nil {
		return nil, err
	}

	limit, offset = normalizeListArgs(limit, offset)

	sb := orm.NewSelectBuilder(r.db, userTableName).
		Columns("id", "name", "email", "age", "created_at").
		OrderByDesc("created_at").
		Limit(limit)
	if offset > 0 {
		sb = sb.Offset(offset)
	}

	rows, err := sb.Query(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*UserEntity
	if err := orm.StructScanAll(rows, &users); err != nil {
		return nil, err
	}
	return users, nil
}
