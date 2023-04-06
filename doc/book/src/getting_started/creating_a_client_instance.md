# Creating a Client Instance

To interact with the API, you'll need to create a client instance. The client instance allows you to configure settings, such as the API key and base URL, and provides methods to interact with the API.

## JavaScript Example

```javascript
const Client = require('client-library-go');
const client = new Client(apiKey);
```

## Go Example:

```go
import (
    "github.com/example/client-library-go"
)

client, err := clientlibrary.NewClient(apiKey)
if err != nil {
    log.Fatalf("Error creating client: %v", err)
}
```