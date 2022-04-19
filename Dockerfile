FROM golang:1.16 as build
RUN apt-get update && apt-get  install curl make
RUN curl -o /aws-iam-authenticator https://amazon-eks.s3-us-west-2.amazonaws.com/1.14.6/2019-08-22/bin/linux/amd64/aws-iam-authenticator \
  && wait \
  && chmod +x /aws-iam-authenticator

COPY . /build/
WORKDIR /build

RUN go env -w GO111MODULE=auto \
   && make

FROM gcr.io/distroless/base
COPY --from=build /go/bin/sloop /sloop
# The copy statement below can be uncommented to reflect changes to any webfiles as compared
# to the binary version of the files in use.
# COPY pkg/sloop/webserver/webfiles /webfiles
COPY --from=build /aws-iam-authenticator /aws-iam-authenticator
ENV PATH="/:${PATH}"
CMD ["/sloop"]
