# Build:  docker build -t wad/classifier:demo classifier/
FROM golang:1.26 AS build
ENV GOTOOLCHAIN=auto CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN go build -trimpath -ldflags="-s -w" -o /classifier .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /classifier /classifier
EXPOSE 9002
ENTRYPOINT ["/classifier"]
