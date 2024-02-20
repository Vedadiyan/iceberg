# iceberg
iceberg is a Kubernetes sidecar proxy that can intercept and filter traffic between the main application container and clients. It provides a flexible way to handle cross-cutting concerns like security, monitoring, and more.

Features
Deploy as sidecar container alongside main app container
Listen on HTTP/HTTPS or Websocket as frontend
Proxy requests to main app as backend
Define filter chains to transform requests and responses
Support filters using different protocols:
- HTTP/HTTPS
- gRPC
- NATS
- Websocket

Filters for:
- Intercepting requests
- Post-processing responses
- Parallel processing without side effects (e.g. logging)
- Exchange headers and body between filter and main traffic
- Ignore exchange mechanism for parallel filters

## Configuration
iceberg is configured via a YAML file specified in the `ICEBERG_CONFIG` environment variable.

### Example configuration:

    apiVersion: iceberg/v1
    spec:
        listen: ":8081"
        resources:
            - name: Some Name
            filterChains:
                - name: authorize
                listener: "nats://[[default-nats]]/test.message.ok.>" 
                level: "request|parallel"
                exchange:
                    headers:
                    - X-User-Token
                    body: false

Key configuration options:
- listen - Address and port iceberg listens on
- resources - Named resources that can be referenced
- filterChains - One or more filter chains to define
- name - Name of filter chain
- listener - Frontend listener protocol/address
- level - Filter level (request, response, parallel)
- exchange - Headers and body to exchange with backend
- frontend - Route to listen on
- backend - Route to proxy to main container

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