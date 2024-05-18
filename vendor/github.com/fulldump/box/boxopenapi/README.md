# BoxOpenAPI

BoxOpenAPI leverages all the API information you have already defined with Box
to generate your OpenAPI specification, including your types.

The function `Spec` returns an `OpenAPI` struct that can be easily serialized to
JSON. However, if you prefer the YAML format, you can use the following code:

```go
import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/itchyny/json2yaml"
)

// ToYaml converts a given object to its YAML representation.
func ToYaml(a any) string {
	var output strings.Builder
	b, _ := json.Marshal(a)
	json2yaml.Convert(&output, bytes.NewReader(b))
	return output.String()
}
```

## Why is this function not included in the library?

The primary reason for not including this function in the library is to avoid
introducing additional dependencies into your software supply chain, which can
be difficult to justify.
