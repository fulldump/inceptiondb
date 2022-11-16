# Find - index not found

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "index": "invented",
    "value": "my-id"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "index": "invented",
    "value": "my-id"
}

HTTP/1.1 500 Internal Server Error
Content-Length: 114
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "error": {
        "description": "Unexpected error",
        "message": "index 'invented' not found, available indexes [my-index]"
    }
}
```


