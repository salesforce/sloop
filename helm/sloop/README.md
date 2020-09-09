# Helm with tiller installation instructions
1. `cd ./helm`
1. `helm install ./sloop`

# This is instructions for folks NOT using tiller (or Spinnaker) for deployment
1. Build image and tag, and replace the current ones in values.yaml, we are working on a common image for people to use, it will be release in a few days.
1. (Optional) Create a namespace in your cluster for sloop to run if you dont have any yet, for example: `kubectl create namespace sloop `
1. (Optional) Examines a chart for possible issues: `helm lint .`
1. Validate helm chart in local when making any helm changes:`helm template .`
1. Write to yaml file: `helm template . --namespace sloop> sloop-test.yaml`
1. Apply the yaml file in cluster: `kubectl -n sloop apply -f sloop-test.yaml`
1. Check if the service is running: `kubectl -n sloop get pods`
1. (Optional) Use port-forward for debugging:  `kubectl port-forward -n sloop service/sloop 8080:80`
1. In your browser, hit `localhost:8080` to see the result, you can use sloop test data to check the view

![SloopTestData](/other/sloop-test.png?raw=true "SloopTestData")
