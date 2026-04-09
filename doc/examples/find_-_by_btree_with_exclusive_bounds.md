# Find - by BTree with exclusive bounds

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "from>": {
        "category": "drink",
        "product": "milk"
    },
    "index": "my-index",
    "limit": 10,
    "skip": 0,
    "to<": {
        "category": "fruit",
        "product": "apple"
    }
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "from>": {
        "category": "drink",
        "product": "milk"
    },
    "index": "my-index",
    "limit": 10,
    "skip": 0,
    "to<": {
        "category": "fruit",
        "product": "apple"
    }
}

HTTP/1.1 200 OK
Content-Length: 48
Content-Type: application/json
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "category": "drink",
    "id": "2",
    "product": "water"
}
```


