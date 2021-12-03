# Plugins

![Plugins](assets/plugins.png)

Plugins allow you to extend Argo Workflows to add new capabilities.

* You don't need to learn Golang, you can write in any language, including Python.
* Simple: a plugin just responds to RPC HTTP requests.
* You can iterate quickly by changing the plugin at runtime.
* You can get your plugin running today, no need to wait 3-5 months for review, approval, merge and an Argo software
  release.

[Executor plugins](executor_plugins.md) written and installed by both users and admins.

## Configuration

Plugins are disabled by default. To enable them, start the controller with `ARGO_PLUGINS=true`, e.g.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workflow-controller
spec:
  template:
    spec:
      containers:
        - name: workflow-controller
          env:
            - name: ARGO_PLUGINS
              value: "true"
```

## Considerations

### Failure Modes

A plugin may fail as follows:

* Connection/socket problems.
* Timeout (1s for controller plugins, 30s for executor plugins).
* Transient error.
* 4xx or 5xx error:
    * 404 error - endpoint will not be invoked again.
    * 503 error - considered a transient error.
* Multiple invocations of the same plugin take too long.

Transient errors are retried, all other errors are considered fatal.

Fatal errors will result in an errored workflow.

