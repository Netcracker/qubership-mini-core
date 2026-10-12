# Removed API

## 1. Overview
The `core-legacy-api` implementation retains the configuration retrieval, property management, and application dicovery endpoints from the original Config Server. 
However, certain endpoints from the original API have been removed because they are no longer required or were not operational.

This document describes the removed endpoints, the reasons for their removal, and the API currently supported by `core-legacy-api`.

## 2. Removed Endpoints

### 2.1. Migration Endpoint

**Status:** Removed.

**Reason:**

The migration endpoint is no longer required because `core-legacy-api` uses Consul as its configuration storage backend and does not support PostgreSQL. Therefore, the database migration functionality provided by the original Config Server is not applicable to the new implementation.

### 2.2. Encryption Endpoints

**Status:** Removed.

**Reason:**

The encryption endpoints were not operational in the original Config Server. They were therefore excluded from `core-legacy-api`.

### 2.3. Resource Controller Endpoints

**Status:** Removed.

**Reason:**

The Resource controller is a built-in controller provided by Spring Cloud Config Server and is listed in documentation. However, its endpoints were not operational in the original Config Server implementation.

Therefore, these endpoints were excluded from `core-legacy-api` and are not supported by the new implementation.

## 3. Currently Supported API

The following endpoints are supported by `core-legacy-api`.

### 3.1. Configuration Retrieval

| Method | Path | Description | Query parameters |
|---|---|---|---|
| GET | `/{application}/{profile}` | Retrieve configuration for an application and profile | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{application}/{profile}/{label}` | Retrieve configuration for an application, profile, and label | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{application}-{profile}.json` | Retrieve configuration in JSON format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{application}-{profile}.properties` | Retrieve configuration in properties format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{application}-{profile}.yaml` | Retrieve configuration in YAML format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{application}-{profile}.yml` | Retrieve configuration in YAML format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{label}/{application}-{profile}.json` | Retrieve labeled configuration in JSON format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{label}/{application}-{profile}.properties` | Retrieve labeled configuration in properties format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{label}/{application}-{profile}.yaml` | Retrieve labeled configuration in YAML format | `resolvePlaceholders` (boolean, default: `true`) |
| GET | `/{label}/{application}-{profile}.yml` | Retrieve labeled configuration in YAML format | `resolvePlaceholders` (boolean, default: `true`) |

### 3.2. Property Management and Application Discovery

These endpoints allow clients to add, update, and delete configuration properties, as well as retrieve the list of available applications.

| Method | Path | Description | Request body |
|---|---|---|---|
| POST | `/{application}/{profile}` | Add or update properties | JSON object, e.g. `{ "key": "value" }` |
| PUT | `/{application}/{profile}` | Add or update properties | JSON object, e.g. `{ "key": "value" }` |
| POST | `/{application}/{profile}/properties-delete` | Delete properties | JSON array of property names, e.g. `["key1", "key2"]` |
| GET | `/applications` | Retrieve the list of applications | None |
