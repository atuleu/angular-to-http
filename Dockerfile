from golang:1.20-alpine as build

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build

FROM alpine

WORKDIR /app

COPY --from=build /app/angular-to-http /app

ENTRYPOINT ["./angular-to-http"]
