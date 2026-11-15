# refract-config-server

Standalone chart for `config_server` - the small HTTP service that stores
`cds.yaml`/`lds.yaml`/`runtime.yaml` pushed by `refractctl` and serves them
to one or more gateways' in-container fetchers. Deployed separately from
the gateway itself, same as running `config_server`'s own
`Dockerfile`/`Makefile` outside Kubernetes.

```bash
helm install my-config-server . --set apiToken=default
```

`default` is for local/dev only. If you enable the Ingress (it is off by
default) the server is reachable from the internet and its token is the only
thing protecting what your gateways route, so use a strong random token from a
Secret instead:

```bash
kubectl create secret generic refract-api-token \
  --from-literal=token="$(openssl rand -hex 32)"

helm install refract-config-server . \
  --set apiTokenExistingSecret.name=refract-api-token \
  --set apiTokenExistingSecret.key=token \
  --set ingress.enabled=true
```

Install the gateway chart with the same Secret (see its README).

Then point a gateway at it - either the `refract` chart's
`configSourceUrl`, or `refractctl config set http.server-url`
directly - using its Service address,
`http://<release-name>.<namespace>.svc.cluster.local:<service.port>`.

## Values

| Key | Description |
| --- | --- |
| `apiToken` | Required; the server refuses to start without one. Prefer `apiTokenExistingSecret` beyond local/dev use. |
| `persistence.enabled` | Persists pushed config across restarts (default `true`, `ReadWriteOnce`). |
| `replicaCount` | Keep at `1` unless `persistence` is backed by a volume multiple pods can safely share - the Deployment strategy is `Recreate` specifically to avoid two writers to the same `ReadWriteOnce` volume. |
