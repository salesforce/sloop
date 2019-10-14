# This is instructions for folks not using tiller (or Spinnaker) for deployment
1. Build image and tag, and replace the current ones in values.yaml, we are working on a common image for people to use, it will be release in a few days.
2. (Optional) Create a namespace in your cluster for sloop to run if you dont have any yet, for example: `kubectl create namespace sloop `
3. (Optional) Examines a chart for possible issues: `helm lint .`
4. Validate helm chart in local when making any helm changes:`helm template .`
5. Write to yaml file: `helm template . --namespace sloop> sloop-test.yaml`
6. Apply the yaml file in cluster: `kubectl -n sloop apply -f sloop-test.yaml`
7. Check if the service is running: `kubectl -n sloop get pods`
8. (Optional) Use port-forward for debugging:  `kc-aws port-forward -n sloop statefulset/sloop 8080 8000`
9. In your browser, hit `localhost:8080` to see the result, you can use sloop test data to check the view

![SloopTestData](/other/sloop-test.png?raw=true "SloopTestData")
