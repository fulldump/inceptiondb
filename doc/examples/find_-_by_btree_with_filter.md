# Find - by BTree with filter

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "filter": {
        "category": "fruit"
    },
    "index": "my-index",
    "limit": 10,
    "skip": 0
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "filter": {
        "category": "fruit"
    },
    "index": "my-index",
    "limit": 10,
    "skip": 0
}

HTTP/1.1 200 OK
Content-Length: 97
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{"category":"fruit","id":"4","product":"apple"}
{"category":"fruit","id":"1","product":"orange"}

```


