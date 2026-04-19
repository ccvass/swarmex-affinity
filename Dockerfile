FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /affinity ./cmd

FROM scratch
COPY --from=build /affinity /affinity
EXPOSE 8080
ENTRYPOINT ["/affinity"]
