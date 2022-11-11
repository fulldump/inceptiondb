# Find - by BTree reverse order

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "index": "my-index",
    "limit": 10,
    "reverse": true,
    "skip": 0
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "index": "my-index",
    "limit": 10,
    "reverse": true,
    "skip": 0
}

HTTP/1.1 200 OK
Content-Length: 192
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{"category":"fruit","id":"1","product":"orange"}
{"category":"fruit","id":"4","product":"apple"}
{"category":"drink","id":"2","product":"water"}
{"category":"drink","id":"3","product":"milk"}

```


