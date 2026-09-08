FROM golang:1.26-alpine AS build
WORKDIR /src
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
RUN CGO_ENABLED=0 go build -o /atlas .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=build /atlas /atlas
EXPOSE 8080
CMD ["/atlas"]
