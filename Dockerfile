FROM golang:1.9.4

RUN apt-get update && apt-get install -y protobuf-compiler
RUN go get golang.org/x/net/context \
        google.golang.org/grpc \
        firebase.google.com/go \
        google.golang.org/api/option \
        github.com/satori/go.uuid \
        github.com/pkg/errors \
        gopkg.in/matryer/try.v1

RUN go get -u github.com/golang/protobuf/proto && \
    cd $GOPATH/src/github.com/golang/protobuf && \
    make all

COPY . $GOPATH/src/github.com/Multy-io/Multy-back
RUN ls -lah $GOPATH/src/github.com/Multy-io/Multy-back  && cd $GOPATH/src/github.com/Multy-io/Multy-back && make build

# from dev
#RUN mkdir -p $GOPATH/src/github.com/Multy-io && \
#    cd $GOPATH/src/github.com/Multy-io && \
#    git clone https://github.com/Multy-io/Multy-back.git && \
#    go get github.com/swaggo/gin-swagger && \
#    cd $GOPATH/src/github.com/Multy-io/Multy-back && \
#    git checkout dev && \
#    git pull origin dev &&\
#    make build

WORKDIR /go/src/github.com/Multy-io/Multy-back/cmd

RUN echo "VERSION 02"

ENTRYPOINT $GOPATH/src/github.com/Multy-io/Multy-back/cmd/multy