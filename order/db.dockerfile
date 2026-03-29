FROM postgres:18

COPY up.sql /docker-entrypoint-initdb.d/1.sql

CMD ["postgres"]
