FROM alpine as build
RUN apk add --no-cache curl
RUN curl -o /aws-iam-authenticator https://amazon-eks.s3-us-west-2.amazonaws.com/1.14.6/2019-08-22/bin/linux/amd64/aws-iam-authenticator \
  && wait \
  && chmod +x /aws-iam-authenticator

FROM gcr.io/distroless/base
COPY sloop /sloop
# The copy statement below can be uncommented to reflect changes to any webfiles as compared
# to the binary version of the files in use.
# COPY pkg/sloop/webserver/webfiles /webfiles
COPY --from=build /aws-iam-authenticator /aws-iam-authenticator
ENV PATH="/:${PATH}"
CMD ["/sloop"]
