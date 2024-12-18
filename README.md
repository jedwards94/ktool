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
  * `-init-timeout-seconds uint`
    * (optional) the time in seconds to wait until the job is running (default 300)
    * example: `-init-timeout-seconds 120`
  * `-keep-on-sig`
    * (optional) don't terminate job or delete config if command receive a SIGINT or SIGTERM
  * `-keep-resources`
    * (optional) don't delete job and config
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
    -template ./examples/example.yaml \
    -file ./examples/example.py 
```
will produce the following output:
```yaml
kind: Job
apiVersion: batch/v1
metadata:
  annotations:
    app.ktool.io/short-uuid: "383e2"
    app.ktool.io/type: script
  creationTimestamp: null
  generateName: ktool-script-
  labels:
    ktool-app: "true"
    script-extension: py
  namespace: default
spec:
  template:
    spec:
      containers:
      - command:
        - python
        - /ktool/script.py
        env:
        - name: PYTHONUNBUFFERED
          value: "1"
        image: python:3
        imagePullPolicy: Always
        name: ktool-script
        resources: {}
        volumeMounts:
        - mountPath: /ktool
          name: ktool-scripts
      restartPolicy: Never
      terminationGracePeriodSeconds: 1
      volumes:
      - configMap:
          defaultMode: 511
          name: ktool-script-383e2
        name: ktool-scripts
  ttlSecondsAfterFinished: 10
status: {}

---
kind: ConfigMap
apiVersion: v1
metadata:
  name: ktool-script-383e2
  namespace: default
data:
  script.py: |-
    import time


    numbers = [10, 20, 30, 40, 50]

    for num in numbers:
        print(f"Number: {num}")
        time.sleep(20)
```
removing the flag `-dry-run` and adding the `-attach` will produce the following console ouput

```sh
❯
❯ ktool script \
    -attach \
    -image python:3 \
    -shell python \
    -template ./examples/example.yaml \
    -file ./examples/example.py
[ktool-script-jpdx2-pnsph][inf]  pod ktool-script-jpdx2-pnsph is running!
[ktool-script-jpdx2-pnsph][inf]  starting log stream for pod ktool-script-jpdx2-pnsph on container ktool-script
[ktool-script-jpdx2-pnsph][log]  Number: 10

[ktool-script-jpdx2-pnsph][log]  Number: 20

[ktool-script-jpdx2-pnsph][log]  Number: 30

[ktool-script-jpdx2-pnsph][log]  Number: 40

[ktool-script-jpdx2-pnsph][log]  Number: 50

[ktool-script-jpdx2-pnsph][inf]  exit code: 0
[ktool-script-jpdx2-pnsph][inf]  reason: Completed
[ktool-script-jpdx2-pnsph][inf]  message: 
```

## TODO
implement new dev-pod command