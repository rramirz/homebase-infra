FROM docker.io/bitnami/postgresql:16.3.0-debian-12-r23

USER root

RUN install_packages git build-essential cmake wget curl ca-certificates libssl-dev libkrb5-dev zlib1g-dev

# Clone TimescaleDB
ENV TIMESCALE_VERSION=2.14.2
RUN git clone --branch ${TIMESCALE_VERSION} https://github.com/timescale/timescaledb.git /tmp/timescaledb && \
    cd /tmp/timescaledb && \
    ./bootstrap -DREGRESS_CHECKS=OFF && \
    cd build && \
    make && make install && \
    cd / && rm -rf /tmp/timescaledb

# Enable the extension
ENV POSTGRESQL_SHARED_PRELOAD_LIBRARIES=timescaledb

USER 1001

