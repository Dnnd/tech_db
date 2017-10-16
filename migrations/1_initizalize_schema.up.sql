CREATE EXTENSION IF NOT EXISTS CITEXT;
CREATE TABLE IF NOT EXISTS users (
  id       SERIAL PRIMARY KEY,
  about    TEXT,
  email    CITEXT UNIQUE       NOT NULL,
  fullname VARCHAR(255)        NOT NULL,
  nickname CITEXT UNIQUE       NOT NULL
);

CREATE TABLE IF NOT EXISTS forums (
  id      SERIAL PRIMARY KEY,
  slug    CITEXT UNIQUE NOT NULL,
  user_id INTEGER REFERENCES users (id),
  title   VARCHAR(255)  NOT NULL
);

CREATE TABLE IF NOT EXISTS threads (
  id        SERIAL PRIMARY KEY,
  forum_id  INTEGER REFERENCES forums (id),
  author_id INTEGER REFERENCES users (id),
  created   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  message   TEXT                     NOT NULL,
  slug      CITEXT UNIQUE,
  title     VARCHAR(255)             NOT NULL
);

CREATE TABLE IF NOT EXISTS posts (
  id        SERIAL PRIMARY KEY,
  forum_id  INTEGER REFERENCES forums (id),
  thread_id INTEGER REFERENCES threads (id),
  author_id INTEGER REFERENCES users (id),
  created   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
  is_edited BOOLEAN                  NOT NULL DEFAULT FALSE,
  message   TEXT                     NOT NULL,
  path      INTEGER []               NOT NULL
);

CREATE TABLE IF NOT EXISTS votes (
  id        SERIAL PRIMARY KEY,
  thread_id INTEGER REFERENCES threads (id),
  user_id   INTEGER REFERENCES users (id),
  voice     INTEGER NOT NULL,
  CONSTRAINT one_voice_per_user UNIQUE (thread_id, user_id)

);

CREATE TABLE IF NOT EXISTS status (
  id     INT PRIMARY KEY,
  forum  INTEGER NOT NULL,
  thread INTEGER NOT NULL,
  "user" INTEGER NOT NULL,
  post   INTEGER NOT NULL
);

CREATE OR REPLACE FUNCTION forum_slug_to_id(slug CITEXT)
  RETURNS INTEGER AS $$
DECLARE fid INTEGER;
BEGIN
  SELECT id
  INTO STRICT fid
  FROM forums
  WHERE forums.slug = forum_slug_to_id.slug;
  RETURN fid;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION build_path(
  IN  parent_id      INTEGER,
  IN  self_thread_id INTEGER,
  OUT self_id        INTEGER,
  OUT path           INT [])
AS $$
BEGIN
  self_id = nextval('posts_id_seq' :: REGCLASS);
  IF parent_id = 0
  THEN
    path = ARRAY [self_id];
    RETURN;
  END IF;
  SELECT posts.path
  INTO path
  FROM posts
  WHERE posts.id = parent_id AND posts.thread_id = self_thread_id;
  IF ARRAY_LENGTH(path, 1) IS NULL
  THEN
    self_id = NULL;
    RETURN;
  END IF;
  path = path || self_id;
  RETURN;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION get_parent(
  IN  path      INT [],
  OUT parent_id INT)
AS $$
DECLARE arr_len INT;
BEGIN
  arr_len = ARRAY_LENGTH(path, 1);
  IF arr_len = 1
  THEN parent_id = 0;
    RETURN;
  ELSE
    parent_id = path [arr_len - 1];
    RETURN;
  END IF;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION user_nickname_to_id(user_nickname CITEXT)
  RETURNS INTEGER AS $$
DECLARE uid INTEGER;
BEGIN
  BEGIN
    SELECT id
    INTO STRICT uid
    FROM users
    WHERE user_nickname = nickname;
    EXCEPTION
    WHEN no_data_found
      THEN RETURN -1;
  END;
  RETURN uid;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION count_inserted_rows()
  RETURNS TRIGGER
AS $$
BEGIN
  IF (TG_TABLE_NAME = 'forums')
  THEN
    UPDATE status
    SET forum = forum + 1;
  ELSIF (TG_TABLE_NAME = 'posts')
    THEN
      UPDATE status
      SET post = post + 1;
  ELSIF (TG_TABLE_NAME = 'threads')
    THEN
      UPDATE status
      SET thread = thread + 1;
  ELSIF (TG_TABLE_NAME = 'users')
    THEN
      UPDATE status
      SET "user" = status."user" + 1;
  END IF;
  RETURN NULL;
END
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION dynamic_less_equal(cond INTEGER,
                                              lhs  TIMESTAMP WITH TIME ZONE,
                                              rhs  TIMESTAMP WITH TIME ZONE)
  RETURNS BOOLEAN AS $$

BEGIN
  CASE

    WHEN num_nulls(cond, lhs, rhs) != 0
    THEN RETURN TRUE;
    WHEN cond = 1
    THEN RETURN lhs <= rhs;
    WHEN cond = 0
    THEN RETURN lhs >= rhs;
  ELSE RETURN TRUE;
  END CASE;
END;
$$ LANGUAGE plpgsql CALLED ON NULL INPUT;

CREATE OR REPLACE FUNCTION dynamic_less(cond BOOLEAN,
                                        lhs  CITEXT,
                                        rhs  CITEXT)
  RETURNS BOOLEAN AS $$
BEGIN
  CASE
    WHEN num_nulls(cond, lhs, rhs) != 0
    THEN RETURN TRUE;
    WHEN cond = TRUE
    THEN RETURN lhs < rhs;
    WHEN cond = FALSE
    THEN RETURN lhs > rhs;
  ELSE RETURN TRUE;
  END CASE;
END;
$$ LANGUAGE plpgsql CALLED ON NULL INPUT;

CREATE OR REPLACE FUNCTION dynamic_less(cond BOOLEAN,
                                        lhs  INTEGER,
                                        rhs  INTEGER)
  RETURNS BOOLEAN AS $$
BEGIN
  CASE
    WHEN num_nulls(cond, lhs, rhs) != 0
    THEN RETURN TRUE;
    WHEN cond = TRUE
    THEN RETURN lhs < rhs;
    WHEN cond = FALSE
    THEN RETURN lhs > rhs;
  ELSE RETURN TRUE;
  END CASE;
END;
$$ LANGUAGE plpgsql CALLED ON NULL INPUT;

CREATE OR REPLACE FUNCTION dynamic_less(cond BOOLEAN,
                                        lhs  INTEGER [],
                                        rhs  INTEGER [])
  RETURNS BOOLEAN AS $$
BEGIN
  CASE
    WHEN num_nulls(cond, lhs, rhs) != 0
    THEN RETURN TRUE;
    WHEN cond = TRUE
    THEN RETURN lhs < rhs;
    WHEN cond = FALSE
    THEN RETURN lhs > rhs;
  ELSE RETURN TRUE;
  END CASE;
END;
$$ LANGUAGE plpgsql CALLED ON NULL INPUT;

CREATE TRIGGER forums_rows_number
  AFTER INSERT
  ON forums
  FOR EACH STATEMENT
EXECUTE PROCEDURE count_inserted_rows();

CREATE TRIGGER users_rows_number
  AFTER INSERT
  ON users
  FOR EACH STATEMENT
EXECUTE PROCEDURE count_inserted_rows();

CREATE TRIGGER posts_rows_number
  AFTER INSERT
  ON posts
  FOR EACH STATEMENT
EXECUTE PROCEDURE count_inserted_rows();

CREATE TRIGGER threads_rows_number
  AFTER INSERT
  ON threads
  FOR EACH ROW
EXECUTE PROCEDURE count_inserted_rows();