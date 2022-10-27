# Get collection - not found

Curl example:

```sh
curl "https://example.com/v1/collections/my-collection"
```


HTTP request/response example:

```http
GET /v1/collections/my-collection HTTP/1.1
Host: example.com



HTTP/1.1 404 Not Found
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


