package controllers

import (
	"tpark_db/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"strconv"
	"tpark_db/models"
	"tpark_db/database"
	"database/sql"
	"tpark_db/database/errors"
	"tpark_db/database/wrappers"

	"time"
	"github.com/go-openapi/strfmt"
	"fmt"
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
		SELECT th.id, COALESCE(th.slug, ''), forums.slug, forums.id
		FROM threads as th JOIN forums ON (th.forum_id = forums.id) WHERE th.id = $1 or th.slug = $2
	`, thread.ID, thread.Slug)
	forumId := 0
	if err := result.Scan(&thread.ID, &thread.Slug, &thread.Forum, &forumId);
		err != nil {
		if err == sql.ErrNoRows {
			tx.Rollback()
		}
		return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
	}

	posts := params.Posts

	now := strfmt.DateTime(time.Now())

	for _, post := range posts {
		creationDate := now
		if post.Created != nil {
			creationDate = *post.Created
		}

		err := tx.Get(post, `
		INSERT INTO posts
			(id, forum_id, thread_id, author_id, "message", path, created)
		SELECT
				id_path.self_id as id,
				v.forum_id,
				v.thread_id as thread_id,
				uid as author_id,
				v.message,
				id_path.path,
				v.created
			  FROM
			  user_nickname_to_id($1) as uid,
			  (SELECT self_id, path FROM build_path(COALESCE($3, 0), $5::integer)) as id_path,
			  (VALUES ($2::integer, $4, $5::integer, $6::timestamptz) ) as v(forum_id, "message", thread_id, created)
		WHERE id_path.self_id IS NOT NULL
		RETURNING *
		`, post.Author, forumId, post.Parent, post.Message, thread.ID, creationDate)

		if err == sql.ErrNoRows {
			tx.Rollback()
			return operations.NewPostsCreateConflict().WithPayload(&models.Error{"Conflict"})
		}
		if errors.CheckForeginKeyViolation(err) {
			return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
		}
		post.Forum = thread.Forum
		post.Thread = thread.ID
	}
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
			get_parent(posts.path) as parent,
			forums.slug as forum,
			users.nickname as author
			FROM
			posts
			JOIN users ON (posts.author_id = users.id and posts.id = $1)
			JOIN forums ON (posts.forum_id = forums.id)
			JOIN threads on (posts.thread_id = threads.id)
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
					  fp.posts,
					  ft.*,
					  users.nickname as "user"
					FROM
					  (SELECT count(*) AS "posts"
					   FROM forums
						 JOIN posts ON (forums.id = posts.forum_id AND forums.id = $1))
						AS fp,
					  (SELECT
						 forums.id,
						 forums.title,
						 forums.slug,
						 forums.user_id,
						 count(CASE WHEN threads.id IS NOT NULL THEN 1 END ) AS "threads"
					   FROM forums
						 LEFT JOIN threads ON (forums.id = threads.forum_id)
					   WHERE forums.id = $1
					   GROUP BY forums.id, forums.slug, forums.user_id)
						AS ft
					  JOIN users ON (ft.user_id = users.id)
				`, post.ForumID)
			}
		case "thread":
			{
				postFull.Thread = &models.Thread{}
				db.Get(postFull.Thread, `
				SELECT
					th.*,
					users.nickname as "author",
					forums.slug as "forum"
				FROM (
					SELECT
					threads.id,
					threads.author_id,
					threads.forum_id,
					threads."message",
					threads.created,
					threads.title,
					COALESCE(threads.slug, '') as "slug",
					COALESCE(SUM(votes.voice), 0) as "votes"
					FROM threads
					LEFT JOIN votes
					ON (threads.id = votes.thread_id)
					WHERE threads.id = $1
					GROUP BY threads.id, threads."message", threads.created, threads.title, threads."slug"
				) as th
				JOIN users ON (th.author_id = users.id)
				JOIN forums ON (th.forum_id = forums.id)
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
			get_parent(updated.path) as parent,
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
	sort := *params.Sort
	threadID, _ := strconv.Atoi(params.SlugOrID)
	if err := db.Get(&threadID, `SELECT id FROM threads WHERE id = $1 or slug = $2`, threadID, params.SlugOrID);
		err != nil {
		return operations.NewThreadGetPostsNotFound().WithPayload(&NotFoundError)
	}

	switch sort {
	default:
		fallthrough
	case "flat":
		db.Select(&posts, fmt.Sprintf(`
		SELECT
			posts.author_id,
			posts.forum_id,
			posts.thread_id as thread,
			posts.created,
			posts.message,
			posts.is_edited as "isEdited",
			posts.id,
			get_parent(posts.path) as parent,
			forums.slug as forum,
			users.nickname as author
			FROM
			posts
			JOIN users ON (posts.author_id = users.id)
			JOIN forums ON (posts.forum_id = forums.id)
			JOIN threads on (posts.thread_id = threads.id)
			WHERE dynamic_less($2, posts.id, $3)
			AND posts.thread_id = $4
			ORDER BY (posts.created , posts.id) %s
			LIMIT $1
		`, sortOrder), params.Limit, isDesc, params.Since, threadID)

	case "tree":
		db.Select(&posts, fmt.Sprintf(`
		WITH since_parent AS ( SELECT path, get_parent(path) as id
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
		  get_parent(posts.path) AS parent,
		  forums.slug            AS forum,
		  users.nickname         AS author
		FROM
		  posts
		  JOIN users ON (posts.author_id = users.id)
		  JOIN forums ON (posts.forum_id = forums.id)
		  JOIN threads ON (posts.thread_id = threads.id)
		WHERE (
				(get_parent(path) = (SELECT id FROM since_parent)  AND dynamic_less($2, posts.id, $3))
				OR dynamic_less($2,path, (SELECT path FROM since_parent))
			  )
			  AND posts.thread_id = $4
		ORDER BY (posts.path, posts.id) %s
		LIMIT $1
		`, sortOrder), params.Limit, isDesc, params.Since, threadID)
	case "parent_tree":
		db.Select(&posts, fmt.Sprintf(`
		WITH since_parent AS ( SELECT path, get_parent(path) as id
							   FROM posts
							   WHERE id = $3)
		SELECT
		  posts.author_id,
		  posts.forum_id,
		  posts.thread,
		  posts.created,
		  posts.message,
		  posts."isEdited",
		  posts.id,
		  get_parent(posts.path) AS parent,
		  forums.slug            AS forum,
		  users.nickname         AS author
		FROM
		  (SELECT posts.author_id,
			 posts.forum_id,
			 posts.thread_id        AS thread,
			 posts.created,
			 posts.message,
			 posts.is_edited        AS "isEdited",
			 posts.id,
			 posts.path,
			 dense_rank() OVER (ORDER BY path[1] %s) as dr
			FROM posts
			WHERE  posts.thread_id = $4
		 	AND (
				(get_parent(path) = (SELECT id FROM since_parent)  AND dynamic_less($2, posts.id, $3))
				OR dynamic_less($2,path, (SELECT path FROM since_parent))
			  )
		  ) as posts
		  JOIN users ON (posts.author_id = users.id)
		  JOIN forums ON (posts.forum_id = forums.id)
		  JOIN threads ON (posts.thread = threads.id)
		  WHERE posts.dr <= $1
		ORDER BY (posts.path, posts.id) %s
		`, sortOrder, sortOrder), params.Limit, isDesc, params.Since, threadID)
	}

	return operations.NewThreadGetPostsOK().WithPayload(posts)
}
