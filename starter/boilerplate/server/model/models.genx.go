package model

import (
	"time"

	"github.com/pkg/errors"
	"github.com/theplant/relay"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type PageInfo = relay.PageInfo

type Company struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `gorm:"index;not null" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"index;not null" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Name        string         `gorm:"not null" json:"name"`
	Description *string        `json:"description,omitempty"`
}

type (
	CompanyEdge       = relay.Edge[*Company]
	CompanyConnection = relay.Connection[*Company]
)

type Task struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `gorm:"index;not null" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"index;not null" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Title       string         `gorm:"not null" json:"title"`
	Description *string        `json:"description,omitempty"`
	Status      TaskStatus     `gorm:"not null" json:"status"`
	AssigneeID  *string        `json:"assigneeId,omitempty"`
}

type (
	TaskEdge       = relay.Edge[*Task]
	TaskConnection = relay.Connection[*Task]
)

type User struct {
	ID          string         `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `gorm:"index;not null" json:"createdAt"`
	UpdatedAt   time.Time      `gorm:"index;not null" json:"updatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deletedAt"`
	Name        string         `gorm:"not null" json:"name"`
	Description *string        `json:"description,omitempty"`
	Age         int            `gorm:"not null" json:"age"`
	CompanyID   string         `gorm:"not null" json:"companyId"`
}

type (
	UserEdge       = relay.Edge[*User]
	UserConnection = relay.Connection[*User]
)

func AutoMigrate(dsn string) error {
	if dsn == "" {
		return errors.New("database.dsn is required")
	}

	db, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn}), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return errors.Wrap(err, "failed to open database connection")
	}

	if err := db.AutoMigrate(&Company{}, &Task{}, &User{}); err != nil {
		return err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return errors.Wrap(err, "failed to get database connection")
	}
	if err := sqlDB.Close(); err != nil {
		return errors.Wrap(err, "failed to close database connection")
	}
	return nil
}
