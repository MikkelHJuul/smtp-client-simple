FROM golang:1.15 as builder
ENV GO111MODULE=on \
        CGO_ENABLED=0 \
        GOOS=linux \
        GOARCH=amd64
WORKDIR /build
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY main.go .
RUN go build -o smtp-client-simple .


FROM scratch
COPY --from=builder /build/smtp-client-simple /
ENV PORT 8080
EXPOSE 8080
ENTRYPOINT ["/smtp-client-simple"]
