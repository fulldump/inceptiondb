# Create index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:createIndex" \
-d '{
    "name": "my-index",
    "options": {
        "field": "id"
    },
    "type": "map"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:createIndex HTTP/1.1
Host: example.com

{
    "name": "my-index",
    "options": {
        "field": "id"
    },
    "type": "map"
}

HTTP/1.1 201 Created
Content-Length: 73
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "name": "my-index",
    "options": {
        "field": "id",
        "sparse": false
    },
    "type": "map"
}
```


