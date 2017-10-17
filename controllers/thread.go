package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/Dnnd/tech_db/database"
	"github.com/Dnnd/tech_db/database/errors"
	"github.com/go-openapi/runtime/middleware"
	"github.com/Dnnd/tech_db/models"
	"github.com/Dnnd/tech_db/database/wrappers"
	"database/sql"
	"strconv"
)

func ThreadCreate(params operations.ThreadCreateParams) middleware.Responder {
	slug := params.Slug
	thread := params.Thread

	db := database.DB.Unsafe()

	threadFromDb := &models.Thread{}

	forum := &wrappers.ForumWrapper{}
	user := &wrappers.UserWrapper{}
	if err := db.Get(threadFromDb, `
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
			WHERE threads.id = $1 or slug = $2
			GROUP BY threads.id, threads."message", threads.created, threads.title, threads."slug"
		) as th
		JOIN users ON (th.author_id = users.id)
		JOIN forums ON (th.forum_id = forums.id)
		`, thread.ID, thread.Slug);
		err != sql.ErrNoRows {
		thread.ID = threadFromDb.ID
		return operations.NewThreadCreateConflict().WithPayload(threadFromDb)
	}

	isForumExists := db.Get(forum, `SELECT id, slug FROM forums where slug = $1`, slug) != sql.ErrNoRows
	isUserExists := db.Get(user, `SELECT id, nickname FROM users where nickname = $1`, thread.Author) != sql.ErrNoRows

	if !isForumExists || !isUserExists {
		return operations.NewForumCreateNotFound().WithPayload(&NotFoundError)
	}
	thread.Author = user.Nickname
	thread.Forum = forum.Slug

	//TODO: something to deal with concurrent INSERT

	result := db.QueryRowx(`
				   INSERT INTO threads(forum_id,author_id,created,"message", slug, title )
 				   VALUES ($1,$2, COALESCE($3, now()), $4, NULLIF($5,''), $6)
 				   RETURNING id, created
 				  `, forum.ID, user.ID, thread.Created, thread.Message, thread.Slug, thread.Title)

	if err := result.Scan(&thread.ID, &thread.Created);
		errors.CheckForeginKeyViolation(err) {
		return operations.NewThreadCreateNotFound().WithPayload(&NotFoundError)
	}

	return operations.NewThreadCreateCreated().WithPayload(thread)
}

func ThreadGetOne(params operations.ThreadGetOneParams) middleware.Responder {
	slugOrId := params.SlugOrID
	threadId := -1
	if id, err := strconv.Atoi(slugOrId); err == nil {
		threadId = id
		slugOrId = ""
	}

	thread := &models.Thread{}
	db := database.DB.Unsafe()
	if err := db.Get(thread, `
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
			WHERE threads.id = $1 or slug = $2
			GROUP BY threads.id, threads."message", threads.created, threads.title, threads."slug"
		) as th
		JOIN users ON (th.author_id = users.id)
		JOIN forums ON (th.forum_id = forums.id)
		`, threadId, slugOrId);
		err != nil {
		return operations.NewThreadGetOneNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewThreadGetOneOK().WithPayload(thread)
}

func ThreadUpdateOne(params operations.ThreadUpdateParams) middleware.Responder {
	slugOrId := params.SlugOrID
	threadId := -1
	if id, err := strconv.Atoi(slugOrId); err == nil {
		threadId = id
		slugOrId = ""
	}
	thread := params.Thread
	updated := &models.Thread{}
	db := database.DB.Unsafe()
	err := db.Get(updated, `
		WITH updated
			AS (
			UPDATE threads
			SET ("message",title ) =
				(COALESCE(NULLIF($1, ''),
				threads."message"),
				COALESCE(NULLIF($2, ''),
				threads.title))
			WHERE threads.id = $3 OR threads.slug = $4
			RETURNING *
		)
		SELECT
			th.*,
			users.nickname as "author",
			forums.slug as "forum"
		FROM (
			SELECT
			updated.id,
			updated.author_id,
			updated.forum_id,
			updated."message",
			updated.created,
			updated.title,
			COALESCE(updated.slug, '') as "slug",
			COALESCE(SUM(votes.voice), 0) as "votes"
			FROM updated
			LEFT JOIN votes
			ON (updated.id = votes.thread_id)
			GROUP BY updated.id, updated.author_id, updated.forum_id, updated."message", updated.created, updated.title, updated."slug"
		) as th
		JOIN users ON (th.author_id = users.id)
		JOIN forums ON (th.forum_id = forums.id)  `,
		thread.Message, thread.Title, threadId, slugOrId)
	if err != nil {
		return operations.NewThreadUpdateNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewThreadUpdateOK().WithPayload(updated)
}

func ThreadVote(params operations.ThreadVoteParams) middleware.Responder {
	thread := &models.Thread{
		ID:   -1,
		Slug: params.SlugOrID,
	}
	if id, err := strconv.Atoi(thread.Slug); err == nil {
		thread.ID = int32(id)
		thread.Slug = ""
	}
	vote := params.Vote
	db := database.DB.Unsafe()

	if thread.ID == -1 {
		err := db.Get(&thread.ID, `SELECT id FROM threads WHERE slug = $1`, thread.Slug)
		if err != nil {
			return operations.NewThreadVoteNotFound().WithPayload(&NotFoundError)
		}
	}
	userId := -1
	if err := db.Get(&userId, `SELECT id FROM users WHERE nickname = $1`, vote.Nickname);
		err != nil {
		return operations.NewThreadVoteNotFound().WithPayload(&NotFoundError)
	}

	_, err := db.Exec(`
		INSERT INTO votes
		(user_id, thread_id, voice)
		VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT one_voice_per_user DO UPDATE SET voice = EXCLUDED.voice
		RETURNING thread_id
	`, userId, thread.ID, vote.Voice)

	if err != nil {
		return operations.NewThreadVoteNotFound().WithPayload(&NotFoundError)
	}

	err = db.Get(thread,
		`
		SELECT
				th.*,
				users.nickname AS "author",
				forums.slug AS "forum"
			FROM (
				SELECT
				threads.id,
				threads.author_id,
				threads.forum_id,
				threads."message",
				threads.created,
				threads.title,
				COALESCE(threads.slug, '') AS "slug",
				COALESCE(SUM(votes.voice), 0) AS "votes"
				FROM threads
				LEFT JOIN votes
					ON (threads.id = votes.thread_id)
				WHERE threads.id = $1
				GROUP BY threads.id, threads.author_id, threads.forum_id, threads."message", threads.created, threads.title, threads."slug"
			) AS th
			JOIN users ON (th.author_id = users.id)
			JOIN forums ON (th.forum_id = forums.id)
		`, thread.ID)

	if err != nil {
		return operations.NewThreadVoteNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewThreadVoteOK().WithPayload(thread)
}
