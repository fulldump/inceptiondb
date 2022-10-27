# Insert - unique index conflict

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:insert" \
-d '{
    "address": "Elm Street 11",
    "id": "my-id",
    "name": "Fulanez"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:insert HTTP/1.1
Host: example.com

{
    "address": "Elm Street 11",
    "id": "my-id",
    "name": "Fulanez"
}

HTTP/1.1 409 Conflict
Content-Length: 103
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "error": {
        "description": "Unexpected error",
        "message": "index conflict: field 'id' with value 'my-id'"
    }
}
```


