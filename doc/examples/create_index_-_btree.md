# Create index - btree

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:createIndex" \
-d '{
    "fields": [
        "category",
        "product"
    ],
    "name": "my-index",
    "type": "btree"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:createIndex HTTP/1.1
Host: example.com

{
    "fields": [
        "category",
        "product"
    ],
    "name": "my-index",
    "type": "btree"
}

HTTP/1.1 201 Created
Content-Length: 109
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "name": "my-index",
    "options": {
        "Fields": [
            "category",
            "product"
        ],
        "Sparse": false,
        "Unique": false
    },
    "type": "btree"
}
```


