package database

import (
	"github.com/jmoiron/sqlx"
	"log"
	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx/reflectx"
	"strings"
	_ "github.com/mattes/migrate/database/postgres"
	_ "github.com/mattes/migrate/source/file"
	"github.com/mattes/migrate"
	"github.com/mattes/migrate/database/postgres"
)

var DB *sqlx.DB

func init() {
	conn, err := sqlx.Open("postgres", "")
	if err != nil {
		log.Fatal(err)
	}
	driver, err := postgres.WithInstance(conn.DB, &postgres.Config{})
	if err != nil {
		log.Fatal(err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres", driver)
	if err != nil {
		log.Fatal(err)
	}
	conn.SetMaxIdleConns(8)
	conn.SetMaxOpenConns(8)
	m.Up()
	conn.Mapper = reflectx.NewMapperFunc("json", strings.ToLower)
	conn.MustExec(`INSERT INTO status (id, forum, post, "user", thread)
							VALUES(0, 0, 0, 0, 0) ON CONFLICT DO NOTHING`)

	DB = conn
}
