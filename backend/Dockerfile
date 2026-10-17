FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /onecrm-api .
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && adduser -D crm
USER crm
WORKDIR /app
COPY --from=build /onecrm-api .
COPY seed.json .
EXPOSE 8080
CMD ["./onecrm-api"]
