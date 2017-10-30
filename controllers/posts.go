package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/go-openapi/runtime/middleware"
	"strconv"
	"github.com/Dnnd/tech_db/models"
	"github.com/Dnnd/tech_db/database"

	"github.com/Dnnd/tech_db/database/errors"
	"github.com/Dnnd/tech_db/database/wrappers"

	"github.com/go-openapi/strfmt"
	"fmt"
	"bytes"
	"github.com/lib/pq"
	"github.com/Dnnd/tech_db/utils"
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
	args := make([]interface{}, 0, len(posts)*7)
	getPostFromDb, _ := tx.Preparex(`
		SELECT
			COALESCE(id_path.self_id, -1) as id,
			author_cred.id as author_id,
			COALESCE(author_cred.nickname, '') as author,
			id_path.path
		FROM
			(SELECT nickname, id FROM users WHERE nickname = $1) as author_cred,
			(SELECT self_id, path FROM build_path(COALESCE($2, 0), $3::integer)) as id_path`)
	for _, post := range posts {
		postWrapper := &wrappers.PostWrapper{}

		getPostFromDb.Get(postWrapper, post.Author, post.Parent, thread.ID)
		if postWrapper.ID == -1 {
			tx.Rollback()
			return operations.NewPostsCreateConflict().WithPayload(&models.Error{"Conflict"})
		}
		if postWrapper.Author == "" {
			tx.Rollback()
			return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
		}
		post.ID = postWrapper.ID
		post.Author = postWrapper.Author
		post.Forum = thread.Forum
		post.Thread = thread.ID
		if post.Created == nil {
			post.Created = &dbtime
		}
		args = append(args,
			post.ID,
			forumId,
			post.Thread,
			postWrapper.AuthorID,
			post.Message,
			pq.Array(postWrapper.Path),
			post.Created)
	}
	insertAll := bytes.NewBufferString(`INSERT INTO posts (id, forum_id, thread_id, author_id, "message", path, created) VALUES ($1,$2,$3,$4,$5,$6,$7)`)
	for i := 8; i <= len(args); i += 7 {
		insertAll.WriteString(fmt.Sprintf(",($%d,$%d,$%d,$%d,$%d,$%d,$%d)", i, i+1, i+2, i+3, i+4, i+5, i+6))
	}
	if _, err := tx.Exec(insertAll.String(), args...); errors.CheckForeginKeyViolation(err) || err != nil {
		tx.Rollback()
		return operations.NewPostsCreateNotFound().WithPayload(&NotFoundError)
	}
	tx.Exec("; UPDATE status SET post = post + $1", len(posts))
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
				  forums_ext.id,
				  forums_ext.slug,
				  forums_ext.title,
				  forums_ext.threads,
	  		      forums_ext."user",
				  count(CASE WHEN posts.id IS NOT NULL THEN 1 END) as "posts"
				FROM (
					   SELECT
						 forums.id,
						 forums.slug,
						 forums.title,
						 users.nickname AS "user",
						 count(CASE WHEN threads.id IS NOT NULL
						   THEN 1 END)  AS "threads"
					   FROM forums
						 JOIN users ON (forums.user_id = users.id AND forums.id = $1)
						 LEFT JOIN threads ON (forums.id = threads.forum_id AND threads.forum_id = $1)

					   GROUP BY forums.id, forums.slug, forums.title, users.nickname
					 ) AS forums_ext
				  LEFT JOIN posts ON (forums_ext.id = posts.forum_id)
				WHERE posts.forum_id =  $1
				GROUP BY forums_ext.id,
				  forums_ext.slug,
				  forums_ext.title,
				   forums_ext."user",
				  forums_ext.threads
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
		queryBuff := bytes.Buffer{}
		queryBuff.WriteString(`
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
				WHERE posts.thread_id = $2`)
		if params.Since != nil {
			utils.GenCompareCondition(&queryBuff, "AND", isDesc, "posts.id", "$3")
		}
		queryBuff.WriteString(" ORDER BY (posts.created , posts.id) ")
		queryBuff.WriteString(sortOrder)
		queryBuff.WriteString(" LIMIT $1")
		if params.Since != nil {
			db.Select(&posts, queryBuff.String(), params.Limit, threadID, params.Since)
		} else {
			db.Select(&posts, queryBuff.String(), params.Limit, threadID)
		}
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
