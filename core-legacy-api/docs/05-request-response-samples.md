# Request/Response Samples

## 1. Overview

This document provides sample requests and responses for the core-legacy-api.

## 2. Sample data

All samples use application `myapp`, profile `dev`, and the following stored properties:

```properties
# Placeholders
server.host=localhost
server.port=8080
app.url=http://${server.host}:${server.port}

# List
servers[0]=alpha
servers[1]=beta

# Conflicting property keys (scalar and parent)
db=primary
db.url=jdbc:postgresql://localhost:5432/myapp

# Conflicting list keys (scalar and list)
tags=x
tags[0]=y
```

## 3.1. Property Management and Application Discovery

### 3.1.1. Add properties

#### POST /{application}/{profile}

**Request**

```http
POST /myapp/dev HTTP/1.1
Content-Type: application/json

{
  "server.host": "localhost",
  "server.port": "8080"
}
```

**Response**

```http
HTTP/1.1 201 Created
```

---

### 3.1.2. Update properties

#### PUT /{application}/{profile}

**Request**

```http
PUT /myapp/dev HTTP/1.1
Content-Type: application/json

{
  "server.port": "9090"
}
```

**Response**

```http
HTTP/1.1 201 Created
```

---

### 3.1.3. Delete properties

#### POST /{application}/{profile}/properties-delete

**Request**

```http
POST /myapp/dev/properties-delete HTTP/1.1
Content-Type: application/json

[
  "db",
  "db.url",
  "tags",
  "tags[0]"
]
```

**Response**

```http
HTTP/1.1 200 OK
```

---

### 3.1.4. List applications

#### GET /applications

**Request**

```http
GET /applications HTTP/1.1
```

**Response**

```json
[
  {
    "name": "myapp",
    "profiles": ["dev", "prod"]
  },
  {
    "name": "global",
    "profiles": ["default"]
  }
]
```

## 3.2. Configuration Retrieval


### Query parameters

| Parameter | Type | Default | Description |
|---|---|---|---|
| `resolvePlaceholders` | boolean | `true` | `true` replaces placeholders such as `${server.port}` with their values. `false` returns them as stored. |

The parameter applies to the JSON, properties, and YAML formats. Each of their samples shows the default response (`resolvePlaceholders=true`), followed by the only part that changes with `resolvePlaceholders=false`. `GET /{application}/{profile}` and `GET /{application}/{profile}/{label}` ignore it and return values as stored.

---

### 3.2.1. Environment JSON

#### GET /{application}/{profile}

| | |
|---|---|
| Description | Retrieve configuration for an application and profile |
| Query parameters | None |
| Output | Flat property sources, no nesting or list conversion |

**Request**

```http
GET /myapp/dev HTTP/1.1
```

**Response**

```json
{
  "name": "myapp",
  "profiles": ["dev"],
  "label": null,
  "version": "0",
  "propertySources": [
    {
      "name": "consul",
      "source": {
        "server.host": "localhost",
        "server.port": "8080",
        "app.url": "http://${server.host}:${server.port}",
        "servers[0]": "alpha",
        "servers[1]": "beta",
        "db": "primary",
        "db.url": "jdbc:postgresql://localhost:5432/myapp",
        "tags": "x",
        "tags[0]": "y"
      }
    }
  ]
}
```

---

#### GET /{application}/{profile}/{label}

| | |
|---|---|
| Description | Retrieve configuration for an application, profile, and label |
| Query parameters | None |
| Output | Flat property sources, no nesting or list conversion |

**Request**

```http
GET /myapp/dev/main HTTP/1.1
```

**Response**

```json
{
  "name": "myapp",
  "profiles": ["dev"],
  "label": "main",
  "version": "0",
  "propertySources": [
    {
      "name": "consul",
      "source": {
        "server.host": "localhost",
        "server.port": "8080",
        "app.url": "http://${server.host}:${server.port}",
        "servers[0]": "alpha",
        "servers[1]": "beta",
        "db": "primary",
        "db.url": "jdbc:postgresql://localhost:5432/myapp",
        "tags": "x",
        "tags[0]": "y"
      }
    }
  ]
}
```

---

### 3.2.2. JSON format

| Method | Path | Description |
|---|---|---|
| GET | `/{application}-{profile}.json` | Retrieve configuration in JSON format |
| GET | `/{label}/{application}-{profile}.json` | Retrieve labeled configuration in JSON format |

Both paths return the same body.

**Request**

```http
GET /myapp-dev.json HTTP/1.1
```

```http
GET /main/myapp-dev.json HTTP/1.1
```

**Response**

```json
{
  "server": {
    "host": "localhost",
    "port": "8080"
  },
  "app": {
    "url": "http://localhost:8080"
  },
  "servers": ["alpha", "beta"],
  "db": "primary",
  "db.url": "jdbc:postgresql://localhost:5432/myapp",
  "tags": "x",
  "tags[0]": "y"
}
```

**With `resolvePlaceholders=false`**

```http
GET /myapp-dev.json?resolvePlaceholders=false HTTP/1.1
```

Only this value changes:

```json
"app": {
  "url": "http://${server.host}:${server.port}"
}
```

---

### 3.2.3. Properties format

| Method | Path | Description |
|---|---|---|
| GET | `/{application}-{profile}.properties` | Retrieve configuration in properties format |
| GET | `/{label}/{application}-{profile}.properties` | Retrieve labeled configuration in properties format |

Both paths return the same body. The output is flat.

**Request**

```http
GET /myapp-dev.properties HTTP/1.1
```

```http
GET /main/myapp-dev.properties HTTP/1.1
```

**Response**

```properties
server.host: localhost
server.port: 8080
app.url: http://localhost:8080
servers[0]: alpha
servers[1]: beta
db: primary
db.url: jdbc:postgresql://localhost:5432/myapp
tags: x
tags[0]: y
```

**With `resolvePlaceholders=false`**

```http
GET /myapp-dev.properties?resolvePlaceholders=false HTTP/1.1
```

Only this line changes:

```properties
app.url: http://${server.host}:${server.port}
```

---

### 3.2.4. YAML format

| Method | Path | Description |
|---|---|---|
| GET | `/{application}-{profile}.yaml` | Retrieve configuration in YAML format |
| GET | `/{application}-{profile}.yml` | Retrieve configuration in YAML format |
| GET | `/{label}/{application}-{profile}.yaml` | Retrieve labeled configuration in YAML format |
| GET | `/{label}/{application}-{profile}.yml` | Retrieve labeled configuration in YAML format |

All four paths return the same body.

**Request**

```http
GET /myapp-dev.yaml HTTP/1.1
```

```http
GET /myapp-dev.yml HTTP/1.1
```

```http
GET /main/myapp-dev.yaml HTTP/1.1
```

```http
GET /main/myapp-dev.yml HTTP/1.1
```

**Response**

```yaml
server:
  host: localhost
  port: '8080'
app:
  url: http://localhost:8080
servers:
- alpha
- beta
db: primary
db.url: jdbc:postgresql://localhost:5432/myapp
tags: x
tags[0]: y
```

**With `resolvePlaceholders=false`**

```http
GET /myapp-dev.yaml?resolvePlaceholders=false HTTP/1.1
```

Only this value changes:

```yaml
app:
  url: http://${server.host}:${server.port}
```

---
