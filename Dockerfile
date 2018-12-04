# Builder image that builds all the multy-back and all node services
# multyio/multy-back-builder has all dependencies cached
# Based on golang:1.9.4
FROM multyio/multy-back-builder as builder
# Get, build and install musl-gcc - a compiler that allows static builds for cgo on alpine
RUN curl http://www.musl-libc.org/releases/musl-1.1.20.tar.gz > musl-1.1.20.tar.gz \
    && tar -xf musl-1.1.20.tar.gz \
    && cd ./musl-1.1.20 \
    && ./configure --prefix=/usr && make && make install

WORKDIR $GOPATH/src/github.com/Multy-io/Multy-back
# Build an image from sources of local directory.
COPY . $GOPATH/src/github.com/Multy-io/Multy-back
RUN go get -v -d ./...
RUN CC=musl-gcc make build -B

# Base image for all images with executable application
# Sets important arguments and labels.
# As for Dec 3 2018 alpine:3.8 had no known vulnerabilities
# golang:1.10-alpine3.8
FROM alpine:3.8 as base
# Common stuff to put into all derived containers
ONBUILD LABEL org.label-schema.schema-version = "1.0"
ONBUILD LABEL org.label-schema.url = "http://multy.io"
ONBUILD LABEL org.label-schema.vcs-url = "https://github.com//multy-io/multy-back"
ONBUILD ARG BUILD_DATE
ONBUILD ARG GIT_COMMIT
ONBUILD ARG GIT_BRANCH
ONBUILD ARG GIT_TAG
ONBUILD LABEL org.label-schema.build-date="$BUILD_DATE"
ONBUILD LABEL org.label-schema.git-branch="$GIT_BRANCH"
ONBUILD LABEL org.label-schema.vcs-ref="$GIT_COMMIT"
ONBUILD LABEL org.label-schema.version="$GIT_TAG"


FROM base as multy-back
LABEL org.label-schema.name = "Multy Back"
WORKDIR /multy
COPY --from=builder /go/src/github.com/Multy-io/Multy-back/cmd/multy-back/multy-back /multy/multy-back
ENTRYPOINT ["/multy/multy-back"]


FROM base as multy-btc-node-service
LABEL org.label-schema.name = "Multy BTC Node service"
WORKDIR /multy
COPY --from=builder /go/src/github.com/Multy-io/Multy-back/cmd/ns-btc/ns-btc /multy/ns-btc
ENTRYPOINT ["/multy/ns-btc"]


FROM base as multy-eth-node-service
LABEL org.label-schema.name = "Multy ETH Node service"
WORKDIR /multy
COPY --from=builder /go/src/github.com/Multy-io/Multy-back/cmd/ns-eth/ns-eth /multy/ns-eth
ENTRYPOINT ["/multy/ns-eth"]
