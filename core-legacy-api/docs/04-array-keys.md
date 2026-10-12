# Arrays Support in JSON and YAML Outputs 

## 1. Array keys (`list[0]`)

### How it works in the Original Config Server

- Keys such as `list[0]` are converted into real lists in the JSON and YAML output.
- A large index such as `list[1000]` makes the server fill the list with `null` elements up to that index.
- A very large index causes an internal server error (HTTP 500).

### How it works in the core-legacy-api

- Array keys are converted into lists, same as in Config Server.
- A limit of **100 000** is applied to the index.
- If the index is above the limit, the key is **not** converted into a list. It is returned as-is, as a flat literal key.

Example:

```properties
list[0]=a
list[1]=b
list[100001]=c
```

Output (JSON):

```json
{
  "list": ["a", "b"],
  "list[100001]": "c"
}
```

## 2. Conflicting keys (`list[0]` and `list`)

### Original Config Server behavior

- Conflicts between a scalar key and an array key are not verified when new property is added.
- When both `list` and `list[0]` exist, JSON and YAML endpoints return a 500 error.

### core-legacy-api behavior

- Conflicting keys do **not** cause an error.
- They are returned as flat literal keys in JSON and YAML endpoints.

Example:

```properties
list=foo
list[0]=bar
```

Output (JSON):

```json
{
  "list": "foo",
  "list[0]": "bar"
}
```
