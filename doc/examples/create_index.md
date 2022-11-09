# Create index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:createIndex" \
-d '{
    "field": "id",
    "name": "my-index",
    "type": "map"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:createIndex HTTP/1.1
Host: example.com

{
    "field": "id",
    "name": "my-index",
    "type": "map"
}

HTTP/1.1 201 Created
Content-Length: 61
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "field": "id",
    "name": "my-index",
    "sparse": false,
    "type": "map"
}
```


