# Drop collection

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:dropCollection"
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:dropCollection HTTP/1.1
Host: example.com



HTTP/1.1 200 OK
Content-Length: 31
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "error": "method_not_allowed"
}
```


