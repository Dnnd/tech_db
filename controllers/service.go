package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/Dnnd/tech_db/database"
	"github.com/go-openapi/runtime/middleware"
	"github.com/Dnnd/tech_db/models"
)

func ServiceClear(params operations.ClearParams) middleware.Responder {
	db := database.DB
	tx, _ := db.Beginx()

	tx.MustExec(`TRUNCATE users, posts, threads, votes, forums RESTART IDENTITY CASCADE`)
	tx.MustExec(`UPDATE status SET(forum, post, "user", thread) =
							( 0, 0, 0, 0)`)
	tx.Commit()
	return operations.NewClearOK()
}

func ServiceStatus(params operations.StatusParams) middleware.Responder {
	db := database.DB
	status := models.Status{}
	db.Get(&status, `SELECT forum, post, "user", thread FROM status WHERE id = 0`)
	return operations.NewStatusOK().WithPayload(&status)
}
