package controllers

import (
	"tpark_db/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"tpark_db/database"
	"tpark_db/models"
	"database/sql"
)

func CreateUser(params operations.UserCreateParams) middleware.Responder {
	user := params.Profile
	nickname := params.Nickname
	user.Nickname = nickname
	db := database.DB
	_, err := db.Exec(`INSERT INTO users
					(about, email, fullname, nickname)
					VALUES
					($1, $2, $3, $4);`, user.About, user.Email, user.Fullname, nickname)
	if err != nil {
		users := models.Users{}
		db.Select(&users, `SELECT about, fullname, nickname, email from users WHERE nickname = $1 or email = $2`, nickname, user.Email)
		return operations.NewUserCreateConflict().WithPayload(users)
	}
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
			   =  ($1, $2, $3)
			   WHERE nickname = $4
			   RETURNING about, nickname, fullname, email`,
		updateFields.About,
		updateFields.Fullname,
		updateFields.Email,
		nickname)
	err := result.StructScan(&user)
	if err == sql.ErrNoRows {
		return operations.NewUserUpdateNotFound().WithPayload(&models.Error{"Not Found"})
	} else if err != nil {
		return operations.NewUserUpdateConflict().WithPayload(&models.Error{"Conflict"})
	}

	return operations.NewUserUpdateOK().WithPayload(&user)
}

func UserGetOne(params operations.UserGetOneParams) middleware.Responder {
	nickname := params.Nickname
	db := database.DB.Unsafe()
	user := models.User{}
	err := db.Get(&user, `SELECT * FROM users WHERE nickname = $1 `, nickname)
	if err != nil {
		return operations.NewUserGetOneNotFound().WithPayload(&models.Error{"Not Found"})
	}
	return operations.NewUserGetOneOK().WithPayload(&user)
}
