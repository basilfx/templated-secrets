# templated-secrets
A Kubernetes operator to template secrets dynamically.

## Introduction
This Kubernetes operator allows you to create secrets dynamically from
templates.

Secrets can be used as environment variables for Pods, using `envFrom` or
`valueFrom`. Sometimes it is desired to create an environemnt variable based on
one or more secrets. While it is possible to use variable substitution to
combine one or more environment variables, it is quite cumbersome to include
this in your Pod spec, especially if you need to rewrite secret names. In
addition, all the variables necessary will pollute the environemt of the Pod.

## Usage
The spec is quite similar to a regular `Secret`.

### Basic usage

```yaml
apiVersion: k8s.basilfx.net/v1alpha1
kind: TemplatedSecret
metadata:
  name: <name>
  namespace: <namespace>
spec:
  data:
    key1: <template>
    key2: <template>
    ...
    keyN: <template>
```

A template is a regular string that can contain one or more variable references
that will be replaced. A variable is defined as
`$(namespace > secretRef > key)`, or `$(secretRef > key)` if the `secretRef` is
within the same namespace as the template.

Using `$(..)` as syntax for variable references does not conflict with the
Sprig templating language as used by Helm and others. Note that advanced
manipulation of variables is not supported.

Although it is possible to use a `TemplatedSecret` just like a regular Secret,
it should not be used as such. Furthermore, the values are treated as regular
strings (not Base64 encoded).

If any of the variables cannot be resolved, the `Secret` will not be created
(or updated). It will be re-queued for reconcilliation. Furthermore, if the
`TemplatedSecret` would overwrite an existing `Secret` (not owned by the
`TemplatedSecret`), it will not continue. In both cases, the status of the
`TemplatedSecret` will be updated.

### Advanced usage
It is also possible to define additional metadata. See the example below:

```yaml
apiVersion: k8s.basilfx.net/v1alpha1
kind: TemplatedSecret
metadata:
  name: <name>
  namespace: <namespace>
spec:
  template:
    type: Opaque
    metadata:
      name: <another-name>
      labels:
        app: some-app
  data:
    key1: <template>
    key2: <template>
    ...
    keyN: <template>
```

## Example
Given the following `Secret`s:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: common-secrets
  namespace: default
type: Opaque
stringData:
  host: example.org
---
apiVersion: v1
kind: Secret
metadata:
  name: other-secrets
  namespace: admin
type: Opaque
stringData:
  token: 123hello456world
```

The `TemplatedSecret` below will deploy and control another `Secret`, based on
the template you define:

```yaml
apiVersion: k8s.basilfx.net/v1alpha1
kind: TemplatedSecret
metadata:
  name: templated-secret
  namespace: default
spec:
  data:
    connectionString: "http://$(common-secrets > host)?token=$(admin > other-secrets > token)"
```

## Building
The easiest way to get started, is to use the Dockerfile and build the
application in Docker. Simply run `docker build . --tag image:tag`.

Alternatively, to run `make run` to build and start this service.

This project depends on the [Operator SDK](https://sdk.operatorframework.io/).
Refer to this project for more information.

## Helm chart
A Helm chart is provided in the `helm/` folder.

## TODO
* Reconcile when any referenced secrets update.
* Template is only applicable for new secrets.

## License
See the `LICENSE.md` file (MIT license).
