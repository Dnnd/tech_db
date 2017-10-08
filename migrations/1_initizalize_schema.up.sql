CREATE EXTENSION IF NOT EXISTS CITEXT;
CREATE TABLE IF NOT EXISTS users (
  id       SERIAL PRIMARY KEY,
  about    TEXT,
  email    VARCHAR(255) UNIQUE NOT NULL,
  fullname VARCHAR(255)        NOT NULL,
  nickname CITEXT UNIQUE       NOT NULL
);

CREATE TABLE IF NOT EXISTS forums (
  id     SERIAL PRIMARY KEY,
  slug   VARCHAR(255) UNIQUE NOT NULL,
  userId INTEGER REFERENCES users (id),
  title  VARCHAR(255)        NOT NULL
);

CREATE TABLE IF NOT EXISTS threads (
  id       SERIAL PRIMARY KEY,
  forum_id INTEGER REFERENCES forums (id),
  author   INTEGER REFERENCES users (id),
  created  TIMESTAMP WITH TIME ZONE NOT NULL,
  message  TEXT                     NOT NULL,
  slug     VARCHAR(255) UNIQUE,
  title    VARCHAR(255)             NOT NULL
);

CREATE TABLE IF NOT EXISTS posts (
  id        SERIAL PRIMARY KEY,
  forum_id  INTEGER REFERENCES forums (id),
  thread_id INTEGER REFERENCES threads (id),
  author    INTEGER REFERENCES users (id),
  isEdited  BOOLEAN    NOT NULL DEFAULT FALSE,
  message   TEXT       NOT NULL,
  path      INTEGER [] NOT NULL
);

CREATE TABLE IF NOT EXISTS votes (
  id        SERIAL PRIMARY KEY,
  thread_id INTEGER REFERENCES threads (id),
  user_id   INTEGER REFERENCES users (id),
  voice     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS status (
  id     INT PRIMARY KEY,
  forum  INTEGER NOT NULL,
  thread INTEGER NOT NULL,
  "user" INTEGER NOT NULL,
  post   INTEGER NOT NULL
);

CREATE OR REPLACE FUNCTION count_inserted_rows()
  RETURNS TRIGGER AS $$
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
      SET ("user") = (status."user" + 1);

  END IF;
  RETURN NULL;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER forums_rows_number
AFTER INSERT ON forums
FOR EACH STATEMENT
EXECUTE PROCEDURE count_inserted_rows();

CREATE TRIGGER users_rows_number
AFTER INSERT ON users
FOR EACH STATEMENT
EXECUTE PROCEDURE count_inserted_rows();

CREATE TRIGGER posts_rows_number
AFTER INSERT ON posts
FOR EACH STATEMENT
EXECUTE PROCEDURE count_inserted_rows();

CREATE TRIGGER threads_rows_number
AFTER INSERT ON threads
FOR EACH ROW
EXECUTE PROCEDURE count_inserted_rows();