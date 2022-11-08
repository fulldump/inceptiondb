# Find - by BTree

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "index": "my-index"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "index": "my-index"
}

HTTP/1.1 200 OK
Content-Length: 192
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{"category":"drink","id":"3","product":"milk"}
{"category":"drink","id":"2","product":"water"}
{"category":"fruit","id":"4","product":"apple"}
{"category":"fruit","id":"1","product":"orange"}

```

