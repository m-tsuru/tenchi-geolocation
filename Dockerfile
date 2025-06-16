FROM golang:1.23 as builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -o tenchi-geolocation main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/tenchi-geolocation ./
COPY --from=builder /app/web ./web
COPY --from=builder /app/main.db ./main.db
EXPOSE 3000
ENV TZ=Asia/Tokyo
CMD ["./tenchi-geolocation"]
