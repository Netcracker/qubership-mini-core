# Conflicting Property Keys

## 1. What Are Conflicting Property Keys?

Two property keys **conflict** when one key is a prefix of another, separated by a dot. In other words, the shorter key is both a complete property and the parent path of a longer key.

For example:

```properties
message=value2
message.text=value1
```

Here, `message` and `message.text` conflict because `message` is both a complete key (with its own value) and a prefix of `message.text`.

## 2. Differences in Handling Conflicting Properties

| System | Behavior |
|--------|----------|
| **Original Config Server** | Validates keys before adding them and **rejects** a new property if its key conflicts with an existing one. |
| **core-legacy-api** | **Allows** conflicting keys to coexist. |

Because conflicting keys can coexist in `core-legacy-api`, special handling is needed during JSON/YAML serialization.

### Example

Consider the following properties:

```properties
message.text=value1
message=value2
```

The keys `message.text` and `message` conflict because one is a prefix of the other.

#### The problem with nested serialization

A conventional nested representation cannot hold both values. `message` would have to be an object (to contain `text`) and a plain string at the same time. One value would override the other, so the result would be either:

```json
{
  "message": {
    "text": "value1"
  }
}
```

or:

```json
{
  "message": "value2"
}
```

Either way, one property is lost.

#### The solution: flat literal keys

To preserve both properties, `core-legacy-api` serializes conflicting keys as **flat literal keys**, without expanding the dots into nested objects:

```json
{
  "message.text": "value1",
  "message": "value2"
}
```

## 3. Keys Starting With a Dot

`core-legacy-api` rejects a write that contains a key starting with a dot, as the original Config Server does: a property name cannot start with a dot. Only the keys in the request are checked; keys already stored are not.

The response is `400 Bad Request` with a plain-text body that lists every such key, and no property from the request is written:

```text
Property keys with errors: 
.foo start with dot
```

A dot inside a key, as in `message.text`, is accepted.
