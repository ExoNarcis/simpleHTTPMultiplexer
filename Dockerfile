# syntax=docker/dockerfile:1
FROM golang:latest
RUN mkdir /httpmulti
WORKDIR /httpmulti
ADD . /httpmulti
COPY *.go ./
COPY go.mod ./
RUN go mod download
RUN go env -w GO111MODULE=on
RUN go build -o /main
EXPOSE 8080
CMD ["/main"]
