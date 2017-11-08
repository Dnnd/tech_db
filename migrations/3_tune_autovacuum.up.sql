SELECT
  p.id,
  p.message,
  p.thread_id              AS thread,
  p.forum_slug :: TEXT     AS forum,
  p.owner_nickname :: TEXT AS author,
  p.created,
  p.isedited,
  p.parent
FROM post p
WHERE p.thread_id = '5001' :: INT AND CASE WHEN '749508' :: INT > -1
  THEN CASE WHEN 'DESC' = 'DESC'
    THEN p.id < $2 :: INT
       ELSE p.id > $2 :: INT END
                                      ELSE TRUE END
ORDER BY p.id DESC, p.thread_id
LIMIT 15;