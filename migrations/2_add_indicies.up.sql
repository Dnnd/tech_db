CREATE INDEX IF NOT EXISTS forum_userid_fkey_idx
  ON forums(user_id);

CREATE INDEX IF NOT EXISTS thread_forumid_fkey_idx
  ON threads (forum_id, id);

CREATE INDEX IF NOT EXISTS thread_forumid_aid_fkey_idx
  ON threads (forum_id, author_id);

CREATE INDEX IF NOT EXISTS thread_fid_created
  ON threads (forum_id, created, id);

CREATE INDEX IF NOT EXISTS thread_authorid_fkey_idx
  ON threads (author_id);

CREATE INDEX IF NOT EXISTS post_threadid_fkey_idx
  ON posts (thread_id, created, id);

CREATE INDEX IF NOT EXISTS post_forumid_authorid_id_idx
  ON posts (forum_id, author_id, id);

CREATE INDEX IF NOT EXISTS post_authorid_fkey_idx
  ON posts (author_id);

CREATE INDEX IF NOT EXISTS post_thid_path_id
  ON posts (thread_id, path, id);

CREATE INDEX IF NOT EXISTS votes_thid_idx
  ON votes (thread_id, voice);