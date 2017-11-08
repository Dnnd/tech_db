package controllers

import (
	"github.com/Dnnd/tech_db/restapi/operations"
	"github.com/Dnnd/tech_db/database"
	"github.com/Dnnd/tech_db/database/errors"
	"github.com/go-openapi/runtime/middleware"
	"github.com/Dnnd/tech_db/models"
	"database/sql"
	"strconv"
	"bytes"
)

func ThreadCreate(params operations.ThreadCreateParams) middleware.Responder {
	thread := params.Thread

	db := database.DB.Unsafe()

	threadFromDb := &models.Thread{}
	if err := db.Get(threadFromDb, `
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
		WHERE threads.id = $1 or threads.slug = $2
		`, thread.ID, thread.Slug);
		err != sql.ErrNoRows {
		thread.ID = threadFromDb.ID
		return operations.NewThreadCreateConflict().WithPayload(threadFromDb)
	}
	var fid, uid int
	res := db.QueryRowx(`
		SELECT u.*,	f.*
		FROM (SELECT id, slug FROM forums WHERE slug = $1) as f,
		(SELECT id, nickname from users WHERE nickname = $2) as u
	`, params.Slug, thread.Author)
	if err := res.Scan(&uid, &thread.Author, &fid, &thread.Forum);
		err != nil {
		return operations.NewForumCreateNotFound().WithPayload(&NotFoundError)
	}

	result := db.QueryRowx(`
				   INSERT INTO threads(forum_id,author_id,created,"message", slug, title )
 				   VALUES ($1, $2, COALESCE($3, now()), $4, NULLIF($5,''), $6)
 				   RETURNING id, created
 				  `, fid, uid, thread.Created, thread.Message, thread.Slug, thread.Title)

	if err := result.Scan(&thread.ID, &thread.Created);
		errors.CheckForeginKeyViolation(err) {
		return operations.NewThreadCreateNotFound().WithPayload(&NotFoundError)
	}

	return operations.NewThreadCreateCreated().WithPayload(thread)
}

func ThreadGetOne(params operations.ThreadGetOneParams) middleware.Responder {
	slugOrId := params.SlugOrID
	gotId := false
	compareCondition := " WHERE threads.slug = $1"
	queryBuff := bytes.Buffer{}
	if _, err := strconv.Atoi(slugOrId); err == nil {
		compareCondition = " WHERE threads.id = $1::int"
		gotId = true
	} else {
		queryBuff.WriteString(`
		WITH thid AS (
			SELECT id FROM threads WHERE threads.slug = $1
		) `)
	}

	thread := &models.Thread{}
	db := database.DB.Unsafe()
	queryBuff.WriteString(`
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
	`)
	if gotId {
		queryBuff.WriteString(`WHERE votes.thread_id = $1::int`)
	} else {
		queryBuff.WriteString(`WHERE votes.thread_id = (SELECT id FROM thid)`)
	}
	queryBuff.WriteString(
		` ) AS v,
				threads
			JOIN users ON (threads.author_id = users.id)
			JOIN forums ON (threads.forum_id = forums.id)`)
	queryBuff.WriteString(compareCondition)
	if err := db.Get(thread, queryBuff.String(), slugOrId);
		err != nil {
		return operations.NewThreadGetOneNotFound().WithPayload(&NotFoundError)
	}
	return operations.NewThreadGetOneOK().WithPayload(thread)
}

func ThreadUpdateOne(params operations.ThreadUpdateParams) middleware.Responder {
	slugOrId := params.SlugOrID
	queryBuffer := bytes.NewBufferString(`
		WITH updated
		AS (
		  UPDATE threads
		  SET ("message",title ) =
		  (COALESCE(NULLIF($1, ''),
					threads."message"),
		   COALESCE(NULLIF($2, ''),
					threads.title))`)

	if _, err := strconv.Atoi(slugOrId); err == nil {
		queryBuffer.WriteString(" WHERE threads.id = $3::int")
	} else {
		queryBuffer.WriteString(" WHERE threads.slug = $3")
	}
	queryBuffer.WriteString(" RETURNING *")
	thread := params.Thread
	updated := &models.Thread{}
	queryBuffer.WriteString(`
	 ) SELECT
		  updated.id,
		  updated.author_id,
		  updated.forum_id,
		  updated."message",
		  updated.created,
		  updated.title,
		  v.votes,
		  COALESCE(updated.slug, '') as "slug",
		  users.nickname as "author",
		  forums.slug as "forum"
		FROM (
			   SELECT
				 COALESCE(SUM(votes.voice), 0) as "votes"
			   FROM votes WHERE votes.thread_id = (SELECT id from updated)
			 ) as v,
		  updated
		  JOIN users ON (updated.author_id = users.id)
		  JOIN forums ON (updated.forum_id = forums.id)
	`)
	db := database.DB.Unsafe()

	err := db.Get(updated, queryBuffer.String(),
		thread.Message, thread.Title, slugOrId)
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
	gotId := false
	if id, err := strconv.Atoi(thread.Slug); err == nil {
		thread.ID = int32(id)
		thread.Slug = ""
		gotId = true
	}

	vote := params.Vote

	queryBuff := bytes.NewBufferString(`
	INSERT INTO votes
	  (user_id, thread_id, voice)
		`)

	tx := database.DB.MustBegin().Unsafe()
	err := error(nil)
	if gotId {
		queryBuff.WriteString(`
		SELECT u.id, $1, $3
		FROM
		(SELECT id from users WHERE nickname = $2) as u
		ON CONFLICT ON CONSTRAINT votes_pkey DO UPDATE SET voice = EXCLUDED.voice
		RETURNING thread_id
	 	`)
		err = tx.Get(&thread.ID, queryBuff.String(), thread.ID, vote.Nickname, vote.Voice)
	} else {
		queryBuff.WriteString(`
		SELECT u.id, t.id, $3
		FROM
		(SELECT id from users WHERE nickname = $1) as u,
		(SELECT id from threads WHERE slug = $2) as t
		ON CONFLICT ON CONSTRAINT votes_pkey DO UPDATE SET voice = EXCLUDED.voice
		RETURNING thread_id
	 	`)
		err = tx.Get(&thread.ID, queryBuff.String(), vote.Nickname, thread.Slug, vote.Voice)
	}

	if err != nil {
		tx.Rollback()
		return operations.NewThreadVoteNotFound().WithPayload(&NotFoundError)
	}

	err = tx.Get(thread,
		`
		SELECT
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
		`, thread.ID)

	if err != nil {
		tx.Rollback()
		return operations.NewThreadVoteNotFound().WithPayload(&NotFoundError)
	}
	tx.Commit()
	return operations.NewThreadVoteOK().WithPayload(thread)
}
