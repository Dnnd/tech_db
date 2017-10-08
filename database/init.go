package database

import (
	"github.com/jmoiron/sqlx"
	"log"
	_ "github.com/lib/pq"
	"github.com/jmoiron/sqlx/reflectx"
	"strings"
)

var DB *sqlx.DB

func init() {
	postgres, err := sqlx.Open("postgres", "")
	if err != nil {
		log.Fatal(err)
	}
	postgres.Mapper = reflectx.NewMapperFunc("json",strings.ToLower)
	postgres.MustExec(`INSERT INTO status (id, forum, post, "user", thread)
							VALUES(0, 0, 0, 0, 0) ON CONFLICT DO NOTHING`)
	DB = postgres
}
