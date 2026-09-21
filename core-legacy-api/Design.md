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

