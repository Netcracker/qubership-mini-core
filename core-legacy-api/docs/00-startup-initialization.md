# Startup Initialization

## Current

On every start, the service adds the default properties that Consul does not have yet. A property that is already
stored keeps its value, including an empty one. A deployment parameter such as `CLOUD_PUBLIC_HOST` therefore changes
only properties that are not stored yet; change a stored one through the API.

## Planned

- Kubernetes CRs based initialization of default properties.
