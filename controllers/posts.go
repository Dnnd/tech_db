package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"strconv"
	"github.com/Dnnd/tech_db/models"
	"github.com/Dnnd/tech_db/database"

	"github.com/Dnnd/tech_db/database/wrappers"

	"github.com/go-openapi/strfmt"
	"fmt"
	"bytes"
	"github.com/lib/pq"
	"github.com/Dnnd/tech_db/utils"
	"sort"
	"github.com/jmoiron/sqlx"
	"strings"
)

func PostCreateMany(params operations.PostsCreateParams) middleware.Responder {
	thread := &models.Thread{
		ID:   -1,
		Slug: params.SlugOrID,
	}
	if id, err := strconv.Atoi(thread.Slug); err == nil {
		thread.ID = int32(id)
		thread.Slug = ""
	}
	tx := database.DB.MustBegin().Unsafe()
	result := tx.QueryRowx(`
		SELECT th.id, COALESCE(th.slug, ''), forums.slug, forums.id, now() as created
		FROM threads as th
		JOIN forums ON (th.forum_id = forums.id)
		WHERE th.id = $1 or th.slug = $2
	`, thread.ID, thread.Slug)

	forumId := 0
	dbtime := strfmt.NewDateTime()
	if err := result.Scan(&thread.ID, &thread.Slug, &thread.Forum, &forumId, &dbtime);
		err != nil {
		tx.Rollback()
		return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
	}
	if len(params.Posts) == 0 {
		tx.Commit()
		return operations.NewPostsCreateCreated().WithPayload(models.Posts{})
	}
	posts := params.Posts
	usersData := make([]struct {
		Id       int
		Nickname string
	}, 0, len(posts))

	postData := make([]struct {
		ParentId int64
		Path     pq.Int64Array
	}, 0, len(posts))

	idSource, _ := tx.Queryx(`SELECT nextval('posts_id_seq'::REGCLASS) AS id
	FROM generate_series(1, $1)`, len(posts))
	defer idSource.Close()
	for _, post := range posts {
		idSource.Next()
		idSource.Scan(&post.ID)
	}
	idSource.Close()

	parents := make([]int64, 0, len(posts))

	for _, post := range posts {
		parents = append(parents, post.Parent)
	}

	tx.Select(&usersData, `SELECT id, nickname FROM users ORDER by nickname`)
	query, args, _ := sqlx.In(`
	SELECT  path, posts.id as parentid
	FROM posts
	WHERE posts.thread_id = ? AND posts.id IN (?)
	ORDER by posts.id;`, thread.ID, parents)
	query = tx.Rebind(query)
	tx.Select(&postData, query, args...)

	insertArgs := make([]interface{}, 0, len(posts)*8)
	for _, post := range posts {

		path := pq.Int64Array{post.ID}
		if post.Parent != 0 {
			postIdx := sort.Search(len(postData), func(i int) bool { return postData[i].ParentId >= post.Parent })
			if postIdx >= len(postData) || postData[postIdx].ParentId != post.Parent {
				tx.Rollback()
				return operations.NewPostsCreateConflict().WithPayload(&models.Error{"Conflict"})
			}
			path = append(postData[postIdx].Path, post.ID)
		}

		authorIdx := sort.Search(len(usersData), func(i int) bool { return strings.ToLower(usersData[i].Nickname) >= strings.ToLower(post.Author) })
		if authorIdx >= len(usersData) || strings.ToLower(usersData[authorIdx].Nickname) != strings.ToLower(post.Author) {
			tx.Rollback()
			return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
		}

		post.Author = usersData[authorIdx].Nickname
		post.Forum = thread.Forum
		post.Thread = thread.ID
		if post.Created == nil {
			post.Created = &dbtime
		}
		insertArgs = append(insertArgs,
			post.ID,
			forumId,
			post.Thread,
			usersData[authorIdx].Id,
			post.Message,
			path,
			post.Created,
			post.Parent)
	}
	insertAll := bytes.NewBufferString(`INSERT INTO posts (id, forum_id, thread_id, author_id, "message", path, created, parent) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`)
	for i := 9; i <= len(insertArgs); i += 8 {
		insertAll.WriteString(fmt.Sprintf(",($%d,$%d,$%d,$%d,$%d,$%d,$%d,$%d)", i, i+1, i+2, i+3, i+4, i+5, i+6, i+7))
	}
	if _, err := tx.Exec(insertAll.String(), insertArgs...); err != nil {
		tx.Rollback()
		return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
	}
	tx.MustExec(`UPDATE forums SET posts_count = posts_count + $1 WHERE id = $2`, len(posts), forumId)
	tx.Commit()
	return operations.NewPostsCreateCreated().WithPayload(posts)
}

func PostGetOne(params operations.PostGetOneParams) middleware.Responder {
	related := params.Related
	postFull := &models.PostFull{}
	post := &wrappers.PostWrapper{}

	db := database.DB.Unsafe()
	err := db.Get(
		post, `
			SELECT posts.author_id,
			posts.forum_id,
			posts.thread_id as thread,
			posts.created,
			posts.message,
			posts.is_edited as "isEdited",
			posts.id,
			posts.parent,
			forums.slug as forum,
			users.nickname as author
			FROM
			posts
			JOIN users ON (posts.author_id = users.id)
			JOIN forums ON (posts.forum_id = forums.id)
			JOIN threads on (posts.thread_id = threads.id)
			WHERE posts.id = $1
	`, params.ID)

	if err != nil {

		return operations.NewPostGetOneNotFound().WithPayload(&NotFoundError)
	}

	for _, item := range related {
		switch item {
		case "user":
			{
				postFull.Author = &models.User{}
				db.Get(postFull.Author, `SELECT * from users WHERE id = $1`, post.AuthorID)
			}
		case "forum":
			{
				postFull.Forum = &models.Forum{}
				db.Get(postFull.Forum, `
				SELECT
				  forums.slug,
				  forums.title,
				  t.tt as "threads",
				  forums.posts_count as "posts",
				  u.nickname as "user"
				FROM (SELECT
						count(*) as tt
					  FROM threads
					  WHERE threads.forum_id = $1
					 ) as t,
				  forums JOIN users u ON forums.user_id = u.id
				WHERE forums.id = $1
				`, post.ForumID)

			}
		case "thread":
			{
				postFull.Thread = &models.Thread{}
				db.Get(postFull.Thread, `
				SELECT
				  threads.id,
				  threads.author_id,
				  threads.forum_id,
				  threads."message",
				  threads.created,
				  threads.title,
				  COALESCE(threads.slug, '') as "slug",
				  users.nickname AS "author",
				  forums.slug AS "forum",
				  v.votes
				FROM (
					   SELECT
						COALESCE(SUM(votes.voice), 0) AS "votes"
					   FROM  votes
					   WHERE votes.thread_id = $1
					 ) AS v,
				  threads
				  JOIN users ON (threads.author_id = users.id)
				  JOIN forums ON (threads.forum_id = forums.id)
				WHERE threads.id = $1
				`, post.Thread)
			}
		}
	}
	postFull.Post = &post.Post

	return operations.NewPostGetOneOK().WithPayload(postFull)
}

func PostUpdate(params operations.PostUpdateParams) middleware.Responder {
	message := params.Post.Message
	id := params.ID
	post := &models.Post{}
	db := database.DB.Unsafe()
	err := db.Get(post, `
			WITH updated AS (
				UPDATE posts SET (message, is_edited)
				= (COALESCE(NULLIF($2,''), message),
				   (VALUES
   					 (CASE $2 WHEN '' THEN is_edited WHEN message THEN is_edited ELSE true END)
  				))
				WHERE id = $1
				RETURNING *
			)
			SELECT
			updated.thread_id as thread,
			updated.created,
			updated.message,
			updated.is_edited as "isEdited",
			updated.id,
			updated.parent,
			forums.slug as forum,
			users.nickname as author
			FROM
			updated
			JOIN users ON (updated.author_id = users.id)
			JOIN forums ON (updated.forum_id = forums.id)
			JOIN threads on (updated.thread_id = threads.id)
	`, id, message)

	if err != nil {
		return operations.NewPostUpdateNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewPostUpdateOK().WithPayload(post)
}

func ThreadsGetPosts(params operations.ThreadGetPostsParams) middleware.Responder {
	db := database.DB.Unsafe()
	posts := models.Posts{}

	isDesc := false
	sortOrder := "ASC"
	if params.Desc != nil && *params.Desc == true {
		sortOrder = "DESC"
		isDesc = true
	}
	sortType := *params.Sort

	queryBuff := &bytes.Buffer{}
	gotId := true
	_, err := strconv.Atoi(params.SlugOrID)
	if err != nil {
		gotId = false
		queryBuff.WriteString(`WITH in_thread_id as (SELECT id FROM threads WHERE slug = $2) `)
	} else {
		queryBuff.WriteString(`WITH in_thread_id as (SELECT $2::int as id) `)
	}

	switch sortType {
	default:
		fallthrough
	case "flat":
		queryBuff.WriteString(`
				SELECT
				  posts.author_id,
				  posts.forum_id,
				  posts.thread_id        AS thread,
				  posts.created,
				  posts.message,
				  posts.is_edited        AS "isEdited",
				  posts.id,
				  posts.parent,
				  f.slug                 AS forum,
				  users.nickname         AS author
				FROM
				  posts
				  JOIN forums f ON (posts.forum_id = f.id)
				  JOIN users ON (posts.author_id = users.id)
		 		WHERE posts.thread_id = (SELECT id from in_thread_id) `)

		if params.Since != nil {
			utils.GenStrictCompareCondition(queryBuff, "AND", isDesc, "posts.id", "$3")
		}
		queryBuff.WriteString("ORDER BY (posts.created , posts.id) ")
		queryBuff.WriteString(sortOrder)
		queryBuff.WriteString(" LIMIT $1")
		if params.Since != nil {
			db.Select(&posts, queryBuff.String(), params.Limit, params.SlugOrID, params.Since)

		} else {
			db.Select(&posts, queryBuff.String(), params.Limit, params.SlugOrID)

		}
	case "tree":
		queryBuff.WriteString(`
		,since_parent AS (SELECT path, parent as id
							   FROM posts
							   WHERE id = $3)
		SELECT
		  posts.author_id,
		  posts.forum_id,
		  posts.thread_id        AS thread,
		  posts.created,
		  posts.message,
		  posts.is_edited        AS "isEdited",
		  posts.id,
		  posts.parent,
		  forums.slug            AS forum,
		  users.nickname         AS author
		FROM
		  posts
		  JOIN users ON (posts.author_id = users.id)
		  JOIN forums ON (posts.forum_id = forums.id)
		  JOIN threads ON (posts.thread_id = threads.id)
		WHERE posts.thread_id = (SELECT id from in_thread_id) `)
		if params.Since != nil {
			utils.GenStrictCompareCondition(queryBuff, "AND ", isDesc, "path", "(SELECT path FROM since_parent)")
		}
		queryBuff.WriteString(" ORDER BY (posts.path, posts.id) ")
		queryBuff.WriteString(sortOrder)
		queryBuff.WriteString(" LIMIT $1")
		db.Select(&posts, queryBuff.String(), params.Limit, params.SlugOrID, params.Since)

	case "parent_tree":
		queryBuff.WriteString(
			`, since_parent AS ( SELECT path, parent as id
							   FROM posts
							   WHERE id = $1)
		SELECT
		  posts.author_id,
		  posts.forum_id,
		  posts.thread_id as thread,
		  posts.created,
		  posts.message,
		  posts.is_edited as "isEdited",
		  posts.id,
		  posts.parent,
		  forums.slug            AS forum,
		  users.nickname         AS author
		FROM
		  (SELECT
			 posts.id,
			 dense_rank() OVER ( ORDER BY get_root(posts.path) `)
		queryBuff.WriteString(sortOrder)
		queryBuff.WriteString(`) as dr
			FROM posts
			WHERE  posts.thread_id = (SELECT id from in_thread_id)
		 	`)
		if params.Since != nil {
			utils.GenStrictCompareCondition(queryBuff, " AND", isDesc, "path", "(SELECT path FROM since_parent)")
		}
		queryBuff.WriteString(`) as p
		  JOIN posts ON (posts.id = p.id)
		  JOIN users ON (posts.author_id = users.id)
		  JOIN forums ON (posts.forum_id = forums.id)
		  JOIN threads ON (posts.thread_id = threads.id)
		  WHERE posts.thread_id = (SELECT id from in_thread_id)`)
		if params.Limit != nil {
			queryBuff.WriteString(` AND p.dr <= $3`)
		}
		queryBuff.WriteString(` ORDER BY (posts.path, posts.id) `)
		queryBuff.WriteString(sortOrder)
		db.Select(&posts, queryBuff.String(),
			params.Since, params.SlugOrID, params.Limit)

	}
	if len(posts) == 0 {
		thid := -1
		if gotId {
			db.Get(&thid, "SELECT id FROM threads WHERE id = $1::int", params.SlugOrID)
		} else {
			db.Get(&thid, "SELECT id FROM threads WHERE slug = $1", params.SlugOrID)
		}
		if thid == -1 {
			return operations.NewThreadGetPostsNotFound().WithPayload(&NotFoundError)
		}
	}
	return operations.NewThreadGetPostsOK().WithPayload(posts)
}
