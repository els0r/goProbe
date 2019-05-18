# NTM-Discovery-Service

API endpoint on which running goProbe processes can register themselves so other applications (for example web frontends) know how and where to reach them.

See this repository's [main README](../../README.md) for how to configure auto-registration for goProbe.

## Usage

All responses will have the form

```json
{
  "data" : "Mixed type holding the content of the response",
  "message" : "User-friendly description of data"
}
```

### List all registered probes

**Definition**

`GET /probes`

**Response**

- `200 OK` on success

```json
[
  {
    "identifier": "infra-core",
    "endpoint" : "192.168.0.2:8080",
    "versions" : [ "v1", "v2" ],
    "keys" : [ "keyA", "keyB" ]
  },
  {
    "identifier": "infra-pi-office",
    "endpoint" : "192.168.1.24:8080",
    "versions" : [ "v1" ],
    "keys" : [ "keyABC" ]
  }
]
```

### Registering a new probe

**Definition**

`POST /probes`

**Arguments**

- `"identifier":string` a globally unique probe identifier (hostname should cut it in most cases)
- `"endpoint":string` the probe's API endpoint
- `"versions":[string]` a list of the probe's supported API versions
- `"keys":[string]` a list of allowed keys to authenticate with the probe's API

**Response**

- `201 Created` on success
- `200 OK` if the resource exists already
- `400 Bad Request` if the arguments weren't supplied correctly

```json
{
    "identifier": "infra-core",
    "endpoint" : "192.168.0.2:8080",
    "versions" : [ "v1", "v2" ],
    "keys" : [ "keyA", "keyB" ]
}
```

## Update an existing probe

**Definition**

`PUT /probes/<identifier>`

**Arguments**

- `"endpoint":string` the probe's API endpoint
- `"versions":[string]` a list of the probe's supported API versions
- `"keys":[string]` a list of allowed keys to authenticate with the probe's API

Notice how the identifier is not part of the arguments, since we are updating an existing probe configuration.
If it is supplied in the body, the application will ignore it.

**Response**

- `404 Not Found` if probe does not exist
- `204 No Content` if nothing was updated
- `200 OK` on successful update of resource

```json
{
    "identifier": "infra-core",
    "endpoint" : "192.168.0.2:8080",
    "versions" : [ "v1", "v2" ],
    "keys" : [ "keyA", "keyB" ]
}
```

## Lookup probe details

`GET /probes/<identifier>`

**Response**

- `404 Not Found` if the probe does not exist
- `200 OK` on success

```json
{
    "identifier": "infra-core",
    "endpoint" : "192.168.0.2:8080",
    "versions" : [ "v1", "v2" ],
    "keys" : [ "keyA", "keyB" ]
}
```

## Delete a probe

**Definition**

`DELETE /probes/<identifier>`

**Response**

- `404 Not Found` if the probe does not exist
- `204 No Content` if the action was successful
