# Create collection

Curl example:

```sh
curl -X POST "https://example.com/v1/collections" \
-d '{
    "name": "my-collection"
}'
```


HTTP request/response example:

```http
POST /v1/collections HTTP/1.1
Host: example.com

{
    "name": "my-collection"
}

HTTP/1.1 201 Created
Content-Length: 47
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "indexes": 0,
    "name": "my-collection",
    "total": 0
}
```


