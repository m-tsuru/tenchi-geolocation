FROM golang:1.23-alpine as builder
WORKDIR /app
RUN apk add --no-cache gcc musl-dev
COPY . .
RUN go mod download
ENV CGO_ENABLED=1
RUN go build -o tenchi-geolocation main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/tenchi-geolocation ./
COPY --from=builder /app/web ./web
COPY --from=builder /app/main.db ./main.db
EXPOSE 3000
ENV TZ=Asia/Tokyo
CMD ["./tenchi-geolocation"]
