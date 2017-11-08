package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"github.com/Dnnd/tech_db/database"
	"database/sql"
	"github.com/Dnnd/tech_db/models"
	"github.com/Dnnd/tech_db/database/errors"
	"bytes"
	"github.com/Dnnd/tech_db/utils"
)

func CreateForum(params operations.ForumCreateParams) middleware.Responder {
	forum := params.Forum
	forumFromBase := models.Forum{}
	db := database.DB

	db.Unsafe().Get(&forumFromBase, `
		SELECT
                  COALESCE(forums_full.slug, '')  AS "slug",
                  COALESCE(forums_full.title, '') AS "title",
                  COALESCE(users.nickname, '')    AS "user",
                  COALESCE(forums_full.posts, 0) as "posts",
                  COALESCE(forums_full.threads, 0) as "threads"
                FROM users
                  LEFT JOIN (
                              SELECT
                                forums.user_id as "user_id",
                                forums.slug as "slug",
                                forums.title as "title",
                                forums.posts_count as "posts",
                                count(threads.*) AS "threads"
                              FROM forums
                                LEFT JOIN threads ON (forums.id = threads.forum_id)
                              WHERE forums.slug = $1
                              GROUP BY forums.user_id, forums.slug, forums.title, forums.posts_count
                            ) AS forums_full
                    ON (users.id = forums_full.user_id)
                WHERE users.nickname = $2`,
		forum.Slug, forum.User)

	if forumFromBase.Slug != "" {
		return operations.NewForumCreateConflict().WithPayload(&forumFromBase)
	}

	if forumFromBase.User == "" {
		return operations.NewForumCreateNotFound().WithPayload(&NotFoundError)
	}

	forum.User = forumFromBase.User
	result := db.Unsafe().QueryRowx(`
						  INSERT INTO forums (slug,title, user_id, posts_count)
						  VALUES($1, $2, user_nickname_to_id($3), 0)
						  RETURNING slug,title, user_id, posts_count as "posts"`,
		forum.Slug, forum.Title, forum.User)

	//TODO: something to deal with concurrent INSERT
	err := result.StructScan(forum)
	if errors.CheckForeginKeyViolation(err) {
		return operations.NewForumCreateNotFound().WithPayload(&NotFoundError)
	}

	forum.Posts = 0
	forum.Threads = 0
	return operations.NewForumCreateCreated().WithPayload(forum)
}

func GetForumDetails(params operations.ForumGetOneParams) middleware.Responder {
	slug := params.Slug
	forum := models.Forum{}
	db := database.DB.Unsafe()
	err := db.Get(&forum, `
	WITH fid AS (
  		SELECT id FROM forums WHERE forums.slug =  $1
	)
		SELECT
		 forums.slug,
		forums.title,
		forums.posts_count as "posts",
		  u.nickname as "user",
		  tc.c as "threads"
		FROM
		  forums
		  JOIN users u ON forums.user_id = u.id,
		 (SELECT count(*) as c FROM  threads t WHERE t.forum_id = (SELECT id from fid)) as tc

		WHERE forums.id = (SELECT id FROM fid)
	`, slug)
	if err == sql.ErrNoRows {
		return operations.NewForumGetOneNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewForumGetOneOK().WithPayload(&forum)
}

func GetThreadsByForum(params operations.ForumGetThreadsParams) middleware.Responder {
	slug := params.Slug
	sortOrder := "ASC"
	isDesc := false
	if params.Desc != nil && *params.Desc == true {
		sortOrder = "DESC"
		isDesc = true
	}
	threads := models.Threads{}
	queryBuff := bytes.NewBufferString(`
			WITH fid AS (SELECT
               id,
               slug
             FROM forums
             WHERE forums.slug = $1 )
			SELECT
			  threads.id,
			  threads.author_id,
			  threads."message",
			  threads.created,
			  u.nickname                 AS "author",
			  (SELECT slug
			   FROM fid)                 AS "forum",
			  threads.title,
			  COALESCE(threads.slug, '') AS "slug",
			  t.votes
			FROM threads
			  JOIN (SELECT
					  threads.id,
					  COALESCE(SUM(v.voice), 0) AS "votes"
					FROM threads
					  LEFT JOIN votes v ON threads.id = v.thread_id
					WHERE threads.forum_id = (SELECT id
											  FROM fid)
					GROUP BY threads.id
				   ) AS t
				ON threads.id = t.id
			  JOIN users u ON threads.author_id = u.id`)
	if params.Since != nil {
		utils.GenCompareConfition(queryBuff, " WHERE", isDesc, `threads.created`, `$3`)
	}
	queryBuff.WriteString(` ORDER BY threads.created `)
	queryBuff.WriteString(sortOrder)
	queryBuff.WriteString(` LIMIT $2`)

	tx := database.DB.MustBegin().Unsafe()
	if params.Since != nil {
		tx.Select(&threads, queryBuff.String(), slug, params.Limit, params.Since)
	} else {
		tx.Select(&threads, queryBuff.String(), slug, params.Limit)
	}
	if len(threads) == 0 {
		forumID := -1
		tx.Get(&forumID, "SELECT id FROM forums WHERE forums.slug = $1", slug)
		if forumID == -1 {
			tx.Rollback()
			return operations.NewForumGetThreadsNotFound().WithPayload(&NotFoundError)
		}
	}
	tx.Commit()
	return operations.NewForumGetThreadsOK().WithPayload(threads)
}

func ForumGetUsers(params operations.ForumGetUsersParams) middleware.Responder {
	desc := params.Desc
	isDesc := false
	sortOrder := "ASC"
	if desc != nil && *desc == true {
		sortOrder = "DESC"
		isDesc = true
	}
	db := database.DB.Unsafe()
	users := models.Users{}
	forumId := 0

	if err := db.Get(&forumId, `SELECT id FROM forums WHERE forums.slug = $1`, params.Slug);
		err != nil {
		return operations.NewForumGetUsersNotFound().WithPayload(&NotFoundError)
	}
	queryBuff := bytes.NewBufferString(`
	SELECT DISTINCT ON (users.nickname)
	  users.nickname,
	  users.email,
	  users.fullname,
	  users.about
	FROM users
	  LEFT JOIN threads t ON (users.id = t.author_id AND t.forum_id = $1)
	  LEFT JOIN posts p ON (users.id = p.author_id AND p.forum_id = $1)
	  WHERE (t.author_id IS NOT NULL OR p.author_id IS NOT NULL)
	`)

	if params.Since != nil {
		utils.GenStrictCompareCondition(queryBuff, " AND ", isDesc, "nickname ", " $3 ")
	}
	queryBuff.WriteString("ORDER BY nickname ")
	queryBuff.WriteString(sortOrder)
	queryBuff.WriteString(" LIMIT $2")
	var err error
	if params.Since != nil {
		err = db.Select(&users, queryBuff.String(), forumId, params.Limit, params.Since)
	} else {
		err = db.Select(&users, queryBuff.String(), forumId, params.Limit)
	}
	if err != nil {
		return operations.NewForumGetUsersNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewForumGetUsersOK().WithPayload(users)
}
