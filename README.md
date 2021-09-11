# Kubernetes Volume Injector

Dynamically add volumes to all pods inside all namespaces with the label `k8s-volume-injector: "true"`.

#### Use cases
* You need to mount /etc/ssl/certs as a hostPath for every pod in order to get pods to trust private CA certificates.
* You need to mount a PersistentVolumeClaim for every pod in order to save logs or data.

#### Requirements
* Kubernetes cluster up & running
* Certmanager needs to be installed

## Installation
1. Create the namespace `k8s-volume-injector`:
```bash
kubectl apply -f deployment/01_namespace.yaml
```
2. Configure your k8s-volume-injector instance:
```bash
vim deployment/02_configmap.yaml # EDIT volumes and volumeMounts as you consider
kubectl apply -f deployment/02_configmap.yaml
```
3. Deploy k8s-volume-injector:
```bash
kubectl apply -f deployment/03_deployment.yaml
```
>NOTE: Wait until pod is running and ready:
>kubectl get po -n k8s-volume-injector
>NAME                                   READY   STATUS    RESTARTS   AGE
>k8s-volume-injector-776fb7cd9f-v9vnz   1/1     Running   0          10s

4. Configure the CA bundle for the webhook:
```bash
caBundle=$( kubectl get secrets -n k8s-volume-injector k8s-volume-injector-cert -o go-template='{{ index .data "ca.crt" }}' )
sed "s@___CA_BUNDLE___@$caBundle@g" deployment/04_webhook.yaml.tmpl > deployment/04_webhook.yaml
```
5. Deploy the webhook:
```bash
kubectl apply -f deployment/04_webhook.yaml
```

## Test webhook
1. Deploy a demo nginx pod:
```bash
kubectl apply -f examples/demo.yaml
```
2. Verify volumes and volumeMounts:
```bash
$ kubectl -n k8s-volume-injector-demo describe po testpod1
Name:         testpod1
Namespace:    k8s-volume-injector-demo
...
Containers:
  web:
    ...
    Mounts:
      /etc/ssl/certs from etc-ssl-certs (ro)
      ...
...
Volumes:
  ...
  etc-ssl-certs:
    Type:          HostPath (bare host directory volume)
    Path:          /etc/ssl/certs
    HostPathType:
...
```
3. Cleanup

## Uninstallation
```bash
kubectl delete -f deployment/04_webhook.yaml # To ensure that the service is not affected
kubectl delete -f deployment/
```
