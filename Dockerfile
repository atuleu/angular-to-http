from golang:1.20-alpine as build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

WORKDIR /app/cmd/angular-to-http

RUN go build

FROM alpine

WORKDIR /app

COPY --from=build /app/cmd/angular-to-http/angular-to-http /app

ENTRYPOINT ["./angular-to-http"]
