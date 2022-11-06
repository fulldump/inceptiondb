# Create index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:createIndex" \
-d '{
    "kind": "map",
    "name": "my-index",
    "parameters": {
        "field": "id"
    }
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:createIndex HTTP/1.1
Host: example.com

{
    "kind": "map",
    "name": "my-index",
    "parameters": {
        "field": "id"
    }
}

HTTP/1.1 201 Created
Content-Length: 51
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "kind": "map",
    "name": "my-index",
    "parameters": null
}
```


