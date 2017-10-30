CREATE INDEX IF NOT EXISTS forum_user_id_fkey
  ON forums USING HASH(user_id);

CREATE INDEX IF NOT EXISTS thread_forum_id_fkey
  ON threads USING HASH(forum_id);

CREATE INDEX IF NOT EXISTS thread_author_id_fkey
  ON threads USING HASH(author_id) ;

CREATE INDEX IF NOT EXISTS post_forum_id_fkey
  ON posts USING HASH(forum_id);

CREATE INDEX IF NOT EXISTS post_thread_id_fkey
  ON posts USING HASH(thread_id);

CREATE INDEX IF NOT EXISTS post_author_id_fkey
  ON posts USING HASH(author_id);

CREATE INDEX IF NOT EXISTS post_forum_author_idx
  ON posts (forum_id, author_id);