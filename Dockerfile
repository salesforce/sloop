FROM golang:1.19 AS build
WORKDIR /go

RUN apt-get update && apt-get install -y curl make

COPY go.mod go.sum ./
RUN go mod tidy

COPY . .
RUN make linux


FROM gcr.io/distroless/base
COPY --from=build /go/bin/sloop /sloop
CMD ["/sloop"]

