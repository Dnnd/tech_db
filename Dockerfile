FROM ubuntu:17.04

RUN apt update  && apt install -y  git wget


RUN wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | apt-key add -
RUN echo "deb http://apt.postgresql.org/pub/repos/apt/ zesty-pgdg main"> /etc/apt/sources.list.d/pgdg.list
RUN apt update && apt install -y postgresql-10

USER postgres
RUN    /etc/init.d/postgresql start &&\
    psql --command " CREATE USER docker WITH SUPERUSER PASSWORD 'docker';" &&\
    createdb -O docker docker &&\
    /etc/init.d/postgresql stop
RUN echo "local all  all  md5" >> /etc/postgresql/10/main/pg_hba.conf.new
RUN cat /etc/postgresql/10/main/pg_hba.conf >> /etc/postgresql/10/main/pg_hba.conf.new
RUN mv /etc/postgresql/10/main/pg_hba.conf.new /etc/postgresql/10/main/pg_hba.conf
RUN echo "unix_socket_directories='/tmp,/var/run/postgresql_sock'" >> /etc/postgresql/10/main/postgresql.conf
RUN echo "synchronous_commit = off" >>  /etc/postgresql/10/main/postgresql.conf
RUN echo "shared_buffers = 256MB" >>  /etc/postgresql/10/main/postgresql.conf
RUN echo "effective_cache_size = 750MB" >>  /etc/postgresql/10/main/postgresql.conf
RUN echo "fsync = off" >>  /etc/postgresql/10/main/postgresql.conf
RUN echo "full_page_writes = off" >>  /etc/postgresql/10/main/postgresql.conf

USER root
RUN wget https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz

RUN tar -C /usr/local -xzf go1.9.1.linux-amd64.tar.gz && \
    mkdir -p go && mkdir -p go/src && mkdir -p go/bin && mkdir -p go/pkg && \
    mkdir -p go/src/

ENV GOPATH /root/go
ENV GOBIN /root/go/bin
ENV PATH $PATH:/root/go/bin:/usr/local/go/bin

WORKDIR $GOPATH/src/github.com/Dnnd/tech_db
ADD . $GOPATH/src/github.com/Dnnd/tech_db
RUN go install ./vendor/github.com/go-swagger/go-swagger/cmd/swagger
RUN swagger generate server --target . --name TechDbForum --spec swagger.yml
RUN go install ./cmd/tech-db-forum-server/

ENV PGHOST	localhost
ENV PORT 5000
ENV HOST 0.0.0.0
ENV PGUSER docker
ENV PGSSLMODE disable
ENV PGDATABASE docker
ENV PGPASSWORD docker
ENV PGHOST /var/run/postgresql_sock

EXPOSE 5000
RUN mkdir -p /var/run/postgresql_sock && chown postgres:postgres /var/run/postgresql_sock
CMD service postgresql start && tech-db-forum-server --scheme http


