# Size - experimental

EXPERIMENTAL!!!

Estimated sizes in bytes: `memory` for the documents, `disk` for the
file and `index.<name>` for every index. It costs O(1): it reads
counters that every write keeps up to date.
					
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
Content-Type: application/json
Date: Mon, 15 Aug 2022 02:08:13 GMT

{
    "disk": 253,
    "index.my-index": 147,
    "memory": 249
}
```


