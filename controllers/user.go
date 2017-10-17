package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"github.com/Dnnd/tech_db/database"
	"github.com/Dnnd/tech_db/models"
	"database/sql"
	"github.com/Dnnd/tech_db/database/errors"
)

func CreateUser(params operations.UserCreateParams) middleware.Responder {
	user := params.Profile
	nickname := params.Nickname
	user.Nickname = nickname
	db := database.DB
	users := models.Users{}
	db.Select(&users, `
		SELECT about, fullname, nickname, email
		FROM users
		WHERE nickname = $1 or email = $2
		`,
		user.Nickname, user.Email)
	if len(users) != 0 {
		return operations.NewUserCreateConflict().WithPayload(users)
	}

	//TODO: something to deal with concurrent INSERT
	db.Exec(`INSERT INTO users
					(about, email, fullname, nickname)
					VALUES
					($1, $2, $3, $4)`, user.About, user.Email, user.Fullname, nickname)

	return operations.NewUserCreateCreated().WithPayload(user)
}

func UpdateUser(params operations.UserUpdateParams) middleware.Responder {
	updateFields := params.Profile
	nickname := params.Nickname
	db := database.DB
	user := models.User{}
	result := db.QueryRowx(
		`UPDATE users
			  SET (about, fullname, email)
			   =  (COALESCE(NULLIF($1, ''), about), COALESCE(NULLIF($2, ''), fullname), COALESCE(NULLIF($3, ''), email))
			   WHERE nickname = $4
			   RETURNING about, nickname, fullname, email`,
		updateFields.About,
		updateFields.Fullname,
		updateFields.Email,
		nickname)
	err := result.StructScan(&user)
	if err == sql.ErrNoRows {
		return operations.NewUserUpdateNotFound().WithPayload(&NotFoundError)
	} else if errors.CheckUniqueViolation(err) {
		return operations.NewUserUpdateConflict().WithPayload(&models.Error{"Conflict"})
	}
	return operations.NewUserUpdateOK().WithPayload(&user)
}

func UserGetOne(params operations.UserGetOneParams) middleware.Responder {
	nickname := params.Nickname
	db := database.DB.Unsafe()
	user := models.User{}
	err := db.Get(&user, `SELECT * FROM users WHERE nickname = $1 `, nickname)
	if err == sql.ErrNoRows {
		return operations.NewUserGetOneNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewUserGetOneOK().WithPayload(&user)
}
