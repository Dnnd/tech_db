FROM ubuntu:rolling

RUN apt update  && apt install -y postgresql-9.6 postgresql-client-9.6

USER postgres
RUN echo 1
RUN    /etc/init.d/postgresql start &&\
    psql --command "CREATE USER docker WITH SUPERUSER PASSWORD 'docker';" &&\
    createdb -O docker docker &&\
    /etc/init.d/postgresql stop

RUN echo "host all  all    0.0.0.0/0  md5" >> /etc/postgresql/9.6/main/pg_hba.conf
RUN echo "listen_addresses='*'" >> /etc/postgresql/9.6/main/postgresql.conf

VOLUME  ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

EXPOSE 5432
CMD ["/usr/lib/postgresql/9.6/bin/postgres", "-D", "/var/lib/postgresql/9.6/main", "-c", "config_file=/etc/postgresql/9.6/main/postgresql.conf"]

USER root
RUN apt install -y git wget
RUN wget https://storage.googleapis.com/golang/go1.9.1.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.9.linux-amd64.tar.gz && \
    mkdir go && mkdir go/src && mkdir go/bin && mkdir go/pkg && \
    mkdir go/src/

ENV GOPATH /root/go
ENV GOROOT /usr/local.go

ENV PATH=${PATH}/bin:/usr/local/go/bin

WORKDIR $GOPATH
ADD /controllers $GOPATH/controllers
ADD /database $GOPATH/database
ADD /vendor $GOPATH/vendor
ADD /restapi/configure_tech_db_forum.go $GOPATH/restapi/configure_tech_db_forum.go
ADD migrations/ $GOPATH/migrations

#CMD ./go/bin/tech_db



