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
	"log"
)

func CreateForum(params operations.ForumCreateParams) middleware.Responder {
	forum := params.Forum
	forumFromBase := models.Forum{}
	db := database.DB

	err := db.Unsafe().Get(&forumFromBase, `
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
                                count(posts.*)   AS "posts",
                                count(threads.*) AS "threads"
                              FROM forums
                                LEFT JOIN threads ON (forums.id = threads.forum_id)
                                LEFT JOIN posts ON (forums.id = posts.forum_id)
                              WHERE forums.slug = $1
                              GROUP BY forums.user_id, forums.slug, forums.title
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
						  INSERT INTO forums (slug,title, user_id)
						  VALUES($1, $2, user_nickname_to_id($3))
						  RETURNING *`,
		forum.Slug, forum.Title, forum.User)

	//TODO: something to deal with concurrent INSERT
	err = result.StructScan(forum)
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
		SELECT
		  fp.posts,
		  ft.*,
		  users.nickname as "user"
		FROM
		  (SELECT count(*) AS "posts"
		   FROM forums
			 JOIN posts ON (forums.id = posts.forum_id AND forums.slug = $1)) AS fp,
		  (SELECT
			 forums.id,
			 forums.title,
			 forums.slug,
			 forums.user_id,
			 count(CASE WHEN threads.id IS NOT NULL THEN 1 END ) AS "threads"
		   FROM forums
			 LEFT JOIN threads ON (forums.id = threads.forum_id)
			WHERE forums.slug = $1
		   GROUP BY forums.id, forums.slug, forums.user_id) AS ft
		  JOIN users ON (ft.user_id = users.id)
	`, slug)
	if err == sql.ErrNoRows {
		return operations.NewForumGetOneNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewForumGetOneOK().WithPayload(&forum)
}

func GetThreadsByForum(params operations.ForumGetThreadsParams) middleware.Responder {
	slug := params.Slug
	since := params.Since
	limit := params.Limit
	sortOrder := "ASC"
	compareWay := 0
	if params.Desc != nil && *params.Desc == true {
		sortOrder = "DESC"
		compareWay = 1
	}

	db := database.DB.Unsafe()
	threads := models.Threads{}
	query, _ := db.Preparex(`
			SELECT
			users.nickname as "author",
			th.*
			FROM (
			SELECT
			threads.id,
			threads.author_id,
			threads."message",
			threads.created,
			threads.title,
			forums.slug as "forum",
			COALESCE(threads.slug, '') as "slug",
			COALESCE(SUM(votes.voice), 0) as "votes"
			FROM threads
			JOIN forums
				ON (threads.forum_id = forums.id AND forums.slug = $1 )
			LEFT JOIN votes
				ON (threads.id = votes.thread_id)
			WHERE dynamic_less_equal($4, threads.created, $3)
			GROUP BY threads.id,threads.author_id, threads."message", threads.created, threads.title, threads.slug, forums.slug
			) as th
			JOIN users ON (th.author_id = users.id)
			ORDER BY th.created ` + sortOrder + ` LIMIT $2`)
	defer query.Close()
	query.Select(&threads, slug, limit, since, compareWay)

	if len(threads) == 0 {
		forumID := -1
		db.Get(&forumID, "SELECT id FROM forums WHERE forums.slug = $1", slug)
		if forumID == -1 {
			return operations.NewForumGetThreadsNotFound().WithPayload(&NotFoundError)
		}
	}
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
		utils.GenCompareCondition(queryBuff, " AND ", isDesc, "nickname ", " $3 ")
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
		log.Println(err)
		return operations.NewForumGetUsersNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewForumGetUsersOK().WithPayload(users)
}
