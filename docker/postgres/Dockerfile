FROM apache/age:release_PG16_1.6.0

RUN apt-get update \
    && apt-get install -y --no-install-recommends build-essential git ca-certificates postgresql-server-dev-16 \
    && git clone --branch v0.8.0 --depth 1 https://github.com/pgvector/pgvector.git /tmp/pgvector \
    && make -C /tmp/pgvector \
    && make -C /tmp/pgvector install \
    && rm -rf /tmp/pgvector \
    && apt-get purge -y --auto-remove build-essential git \
    && rm -rf /var/lib/apt/lists/*

COPY init/01-bootstrap.sql /docker-entrypoint-initdb.d/01-bootstrap.sql
