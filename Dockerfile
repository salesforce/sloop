FROM golang:1.13 AS build

WORKDIR /sloop
COPY go.mod go.sum ./

RUN go mod download

COPY pkg ./pkg

RUN curl -o /sloop/aws-iam-authenticator https://amazon-eks.s3-us-west-2.amazonaws.com/1.14.6/2019-08-22/bin/linux/amd64/aws-iam-authenticator \
  && wait \
  && chmod +x /sloop/aws-iam-authenticator
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s" -installsuffix cgo -o sloop ./pkg/sloop

FROM gcr.io/distroless/base
COPY --from=build /sloop/sloop /sloop
COPY --from=build /sloop/pkg/sloop/webfiles /pkg/sloop/webfiles
COPY --from=build /sloop/aws-iam-authenticator /aws-iam-authenticator
ENV PATH="/:${PATH}"
CMD ["/sloop"]
