# Drop index

Curl example:

```sh
curl -X POST "https://example.com/v1/collections/my-collection:dropIndex" \
-d '{
    "name": "my-index"
}'
```


HTTP request/response example:

```http
POST /v1/collections/my-collection:dropIndex HTTP/1.1
Host: example.com

{
    "name": "my-index"
}

HTTP/1.1 204 No Content
Date: Mon, 15 Aug 2022 02:08:13 GMT


```


