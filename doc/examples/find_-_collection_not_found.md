# Find - collection not found

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/your-collection:find" \
-d '{}'
```


HTTP request/response example:

```http
POST /v1/collections/your-collection:find HTTP/1.1
Host: example.com

{}

HTTP/1.1 500 Internal Server Error
Content-Length: 78
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "error": {
        "description": "Unexpected error",
        "message": "collection not found"
    }
}
```


