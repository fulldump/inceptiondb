# List collections

Curl example:

```sh
curl "https://example.com/v1/collections"
```


HTTP request/response example:

```http
GET /v1/collections HTTP/1.1
Host: example.com



HTTP/1.1 200 OK
Content-Length: 37
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

[
    {
        "name": "my-collection",
        "total": 0
    }
]
```


