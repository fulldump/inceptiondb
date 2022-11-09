# Size - experimental

EXPERIMENTAL!!!

This will probably be removed, it is extremely inefficient.
					
Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:size"
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:size HTTP/1.1
Host: example.com



HTTP/1.1 200 OK
Content-Length: 47
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "disk": 640,
    "index.my-index": 192,
    "memory": 248
}
```


