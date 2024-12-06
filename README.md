# KTool
a simple CLI to help develop and debug kubernetes related things

## Capabilities
- Run local script loaded as a ConfigMap in a Job and stream it's logs to `stdout`
- More to come


## Installation
### Go
```bash
make install
```
### Docker
```bash
make install-docker
```

## Commands and Flags
### Global flags
  * `-kubeconfig string`
    * (optional) absolute path to the kubeconfig file (default "$HOME/.kube/config")

## `script`
Run a local script loaded in a ConfigMap in a Job
### Flags

  * `-args string`
    * (optional) the args to pass to the script file
    * example: `-args '-foo oof -bar rab -baz "some text"'`
  * `-attach`
    * (optional) follow logs
  * `-dry-run`
    * (optional) print the kubernetes manifests but doesn't run anything
  * `-file string`
    * (required) the script file to run
    * example: `-file ./script.py`
  * `-image string`
    * (required) the image where the script will be runned
    * example: `-image python:3.10`
  * `-namespace string`
    * (optional) the namespace where to run the script (default "default")
  * `-shell string`
    * (optional) the shell to use (default "/bin/sh -c")
    * example: `-shell python`
  * `-template string`
    * (optional) the kubernetes Job template to use
    * example: `-template job-with-annotations.yaml`
## Example
### Simple example
```sh
ktool script \
    -dry-run \
    -image python:3 \
    -shell python \
    -file ./examples/example.py
```
will produce the following output:
```yaml
kind: Job
apiVersion: batch/v1
metadata:
  namespace: default
  generateName: ktool-script-
  annotations:
    app.ktool.io/short-uuid: e4d80
    app.ktool.io/type: script
  creationTimestamp: null
  labels:
    ktool-app: "true"
    script-extension: py
spec:
  template:
    metadata:
      creationTimestamp: null
    spec:
      containers:
      - command:
        - python
        - /ktool/script.py
        image: python:3
        imagePullPolicy: Always
        name: ktool-script
        resources: {}
        volumeMounts:
        - mountPath: /ktool
          name: ktool-scripts
      restartPolicy: Never
      volumes:
      - configMap:
          defaultMode: 511
          name: ktool-script-e4d80
        name: ktool-scripts
  ttlSecondsAfterFinished: 10
status: {}

---
kind: ConfigMap
apiVersion: v1
metadata:
  creationTimestamp: null
  name: ktool-script-e4d80
  namespace: default
data:
  script.py: |
    import time
    print("hello world")
    time.sleep(10)
    print("something more")
    time.sleep(3)
    print("goodbye!")
```
removing the flag `-dry-run` and adding the `-attach` will produce the following console ouput

```sh
❯
ktool script \
    -attach \
    -image python:3 \
    -shell python \
    -file ./examples/example.py
[script][inf]  created configmap ktool-script-4b74f on namespace default
[script][inf]  created job ktool-script-p47n2 on namespace default
[ktool-script-p47n2-m47pc][inf]  waiting for pod ktool-script-p47n2-m47pc to be running
[ktool-script-p47n2-m47pc][inf]  pod ktool-script-p47n2-m47pc is running!
[ktool-script-p47n2-m47pc][inf]  starting log stream for pod ktool-script-p47n2-m47pc on container ktool-script
[ktool-script-p47n2-m47pc][log]  hello world
[ktool-script-p47n2-m47pc][log]  something more
[ktool-script-p47n2-m47pc][log]  goodbye!
[ktool-script-p47n2-m47pc][inf]  exit code: 0
[ktool-script-p47n2-m47pc][inf]  reason: Completed
[ktool-script-p47n2-m47pc][inf]  logs ended!
[script][inf]  deleted job ktool-script-p47n2 on namespace default
[script][inf]  deleted configmap ktool-script-4b74f on namespace default
```