# Core-legacy-api design

## 1. Startup Initialization
### Current

-  init_properties.go initialize default properties for the `global` application, `default` profile at the server startup.

### Planned

- Kubernetes CRs based initialization of default properties.

## 2. API

### Config Controller

| Method | Path | Endpoint | Query Params | Body |
|---|---|---|---|---|
| GET | `/{application}/{profile}` | Name Profiles Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{application}/{profile}/{label}` | Name Profiles Label Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{application}-{profile}.json` | Name Profiles Json Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{application}-{profile}.properties` | Name Profiles Properties Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{application}-{profile}.yaml` | Name Profiles Yaml Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{application}-{profile}.yml` | Name Profiles Yml Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{label}/{application}-{profile}.json` | Label Name Profiles Json Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{label}/{application}-{profile}.properties` | Label Name Profiles Properties Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{label}/{application}-{profile}.yaml` | Label Name Profiles Yaml Get | `resolvePlaceholders` (boolean, default: true) | — |
| GET | `/{label}/{application}-{profile}.yml` | Label Name Profiles Yml Get | `resolvePlaceholders` (boolean, default: true) | — |
| POST | `/{application}/{profile}` | Add new properties | — | JSON object (`{ "key": "value", ... }`) |
| PUT | `/{application}/{profile}` | Add new properties | — | JSON object (`{ "key": "value", ... }`) |
| POST | `/{application}/{profile}/properties-delete` | Delete properties | — | JSON array of property names (`["key1", "key2"]`) |
| GET | `/applications` | Get applications list | — | — |


## 2. Differences from Config Server

### Conflicting Keys

Unlike Config Server, Core-legacy-api does not reject conflicting property keys such as `a` and `a.b`.

This is intentional. Consul KV itself can safely store these keys, and the API allows them to coexist.

When properties contain conflicting keys, JSON/YAML serialization represents the conflicting property as a flat literal key instead of interpreting dots as nested properties. For example:

```text
message.text = "value1"
message = "value2"
```

is represented so that `message.text` remains a literal key rather than being converted to:

```json
{
  "message": {
    "text": "value1"
  }
}
```

This avoids ambiguity during serialization while preserving the properties stored in Consul.

### Locks

Unlike the Config Server implementation, Core-legacy-api does not acquire an application-level lock for write operations.

Consul KV serializes individual write operations, so concurrent writes do not result in corrupted or partially merged values. If two requests write different values to the same key concurrently, the resulting value will be one of the two writes.

An application-level lock would only be required for a read-modify-write operation where the application reads the current state, makes a decision based on that state, and then writes the result.

The previous implementation required such a read-modify-write sequence to validate conflicting property keys. Since Core-legacy-api now allows conflicting keys to be stored, this validation is no longer performed and the corresponding lock is not required.
