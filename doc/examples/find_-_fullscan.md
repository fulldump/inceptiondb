# Find - fullscan

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:find" \
-d '{
    "filter": {
        "name": "Fulanez"
    },
    "limit": 1,
    "mode": "fullscan",
    "skip": 0
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:find HTTP/1.1
Host: example.com

{
    "filter": {
        "name": "Fulanez"
    },
    "limit": 1,
    "mode": "fullscan",
    "skip": 0
}

HTTP/1.1 200 OK
Content-Length: 58
Content-Type: text/plain; charset=utf-8
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "address": "Elm Street 11",
    "id": "my-id",
    "name": "Fulanez"
}
```


