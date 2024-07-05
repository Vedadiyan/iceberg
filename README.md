![Go Version](https://img.shields.io/badge/Go-%3E%3D%201.20-%23007d9c)
[![Go report](https://goreportcard.com/badge/github.com/vedadiyan/iceberg)](https://goreportcard.com/report/github.com/vedadiyan/iceberg)

<p align="center">
  <img width="250px" src="https://cdn-icons-png.flaticon.com/512/6362/6362931.png" />
</p>

<p align="center">
  <b align="center">Iceberg (K8s Sidecar Solution)</b>
</p>


# iceberg
iceberg is a Kubernetes sidecar proxy that can intercept and filter traffic between the main application container and clients. It provides a flexible way to handle cross-cutting concerns like security, monitoring, and more.

## Features
- Deploy as sidecar container alongside main app container
- Listen on HTTP/HTTPS or Websocket as frontend
- Proxy requests to main app as backend
- Define filter chains to transform requests and responses

Support filters using different protocols:
- HTTP/HTTPS
- gRPC (SUPPORT DROPPED)
- NATS
- Websocket (In Development)

Filters for:
- Intercepting requests
- Post-processing responses
- Parallel processing without side effects (e.g. logging)
- Exchange headers and body between filter and main traffic
- Ignore exchange mechanism for parallel filters

## Configuration
iceberg is configured via a YAML file specified in the `ICEBERG_CONFIG` environment variable.

### Example configuration:

    apiVersion: apps/v1
    metadata:
      name: test
    spec:
      listen: ''
      resources:
        main-api:
          frontend: ''
          backend: ''
          method: ''
          use:
            cache:
              addr: 'jetstream://[[default_nats]]/bucket_name'
              ttl: 30s
              key: 'test_${:route_value}_${?query_param}_${body}_${method}'
            cors: default
          filters:
            - name: request-log
              addr: 'jetstream://[default_nats]/abc'
              level: request
              timeout: 30s
              onError: default
              async: false
              await: []
              exchange:
                headers:
                  - X-Test-Header
                body: true
              next:
                - name: test
                  addr: 'nats://[default_nats]/test'
                  onError: default
                  timeout: 30s
                  async: true
                  await: []
                - name: test2
                  addr: 'nats://[default_nats]/test2'
                  timeout: 30s
                  onError: default
                  async: false
                  await:
                    - test

To specify a host via environment variable, use [[envvar]] syntax.

## Deployment

Deploy iceberg sidecar container in pod alongside main app container. Main container ports should not be exposed directly.

Example pod spec:

    spec:
        containers:
            - name: main-app
            # main app image
            - name: iceberg 
            image: iceberg
            env:
                - name: ICEBERG_CONFIG
                value: |
                    # iceberg config here
            ports:
                - containerPort: 8081
        
This exposes the iceberg proxy on port 8081 to handle all incoming traffic to the pod. Main app container is accessed internally as the backend.

## Usage

With iceberg deployed as sidecar, all traffic to the pod will be proxied through iceberg and filtered based on configured chains.

Add filter chains to:
- Validate requests
- Enrich requests with data from other services
- Scrub response data
- Log/monitor requests without side effects

And more!
