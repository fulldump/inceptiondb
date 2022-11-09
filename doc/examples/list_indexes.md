# List indexes

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:listIndexes"
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:listIndexes HTTP/1.1
Host: example.com



HTTP/1.1 200 OK
Content-Length: 47
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

[
    {
        "name": "my-index",
        "options": null,
        "type": ""
    }
]
```


