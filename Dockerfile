FROM alpine:3.10
ADD sloop /bin/
# Place webfiles in the same relative path from github to the root of the container
# which is the default current working directory
ADD ./pkg/sloop/webfiles/ /pkg/sloop/webfiles/
CMD ["/bin/sloop"]
