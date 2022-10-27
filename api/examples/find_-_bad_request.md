# Find - bad request

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "mode": "{invalid}"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "mode": "{invalid}"
}

HTTP/1.1 400 Bad Request
Content-Length: 121
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "error": {
        "description": "Unexpected error",
        "message": "bad mode '{invalid}', must be [fullscan|unique]. See docs: TODO"
    }
}
```


