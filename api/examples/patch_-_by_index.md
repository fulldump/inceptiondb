# Patch - by index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:patch" \
-d '{
    "field": "id",
    "mode": "unique",
    "patch": {
        "name": "Pedro"
    },
    "value": "3"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:patch HTTP/1.1
Host: example.com

{
    "field": "id",
    "mode": "unique",
    "patch": {
        "name": "Pedro"
    },
    "value": "3"
}

HTTP/1.1 200 OK
Content-Length: 0
Date: Mon, 15 Aug 2022 02:08:13 GMT


```


