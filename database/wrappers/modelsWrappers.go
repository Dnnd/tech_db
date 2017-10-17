package wrappers

import (
	"github.com/Dnnd/tech_db/models"
)

type ForumWrapper struct {
	ID int
	models.Forum
}
type UserWrapper struct {
	ID int
	models.User
}
type ThreadWrapper struct {
	ForumID  int `json:"forum_id"`
	AuthorID int `json:"author_id"`
	models.Thread
}
type PostWrapper struct {
	ForumID  int `json:"forum_id"`
	AuthorID int `json:"author_id"`
	models.Post
}
