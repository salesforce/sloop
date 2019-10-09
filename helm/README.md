# This is instructions for folks not using tiller (or Spinnaker) for deployment
1. Build image and tag, and replace the current ones in values.yaml, we are working an image for people to use, it will be release in a few days.
2. (Optional) Create a namespace in your cluster for sloop to run if you dont have any namespace yet, for example: `kubectl create namespace sloop `
3. Validate your helm chart in local to make sure there is no mistake:`helm template .`
4. Write to yamil file: `helm template . --namespace sloop> sloop-test.yaml`
5. Apply the yaml file in your cluster: `kubectl -n sloop apply -f sloop-test.yaml`
6. Check if the service is running: kubectl -n sloop get pods
7. (Optional) Use port-forward for debugging:  `kc-aws port-forward -n sloop statefulset/sloop 8080 8000`
8. In your browser, hit `localhost:8080` to see the result, you can set the name filter to be sloop to see our testing data
