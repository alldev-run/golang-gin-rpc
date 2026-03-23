package model

import (
	"context"
	"errors"
	"time"

	"alldev-gin-rpc/pkg/db/mysql"
	"alldev-gin-rpc/pkg/db/orm"
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

func NewMySQLClientFromConfig(cfg mysql.Config) (*mysql.Client, error) {
	return mysql.New(cfg)
}

func (r *UserRepository) Create(ctx context.Context, u *UserEntity) error {
	if u == nil {
		return errors.New("user is nil")
	}
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now()
	}
	_, err := orm.NewInsertBuilder(r.db, "users").Sets(map[string]interface{}{
		"id":         u.ID,
		"name":       u.Name,
		"email":      u.Email,
		"age":        u.Age,
		"created_at": u.CreatedAt,
	}).Exec(ctx)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*UserEntity, error) {
	rows, err := orm.NewSelectBuilder(r.db, "users").
		Columns("id", "name", "email", "age", "created_at").
		Eq("id", id).
		Limit(1).
		Query(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var u UserEntity
	if err := orm.StructScan(rows, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, limit, offset int) ([]*UserEntity, error) {
	sb := orm.NewSelectBuilder(r.db, "users").
		Columns("id", "name", "email", "age", "created_at").
		OrderByDesc("created_at")
	if limit > 0 {
		sb = sb.Limit(limit)
	}
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
