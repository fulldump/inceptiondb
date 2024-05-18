package boxopenapi

/**
Here are some of the types from the OpenAPI 3.1.0 specification represented as
structs. The aim is not to cover the entire specification but to provide users
of this library with an easy way to modify the generated spec.
*/

// OpenAPI follows the spec from https://swagger.io/specification/#openapi-object
type OpenAPI struct {
	Openapi           string   `json:"openapi"`
	Info              Info     `json:"info"`
	JsonSchemaDialect string   `json:"jsonSchemaDialect,omitempty"`
	Servers           []Server `json:"servers,omitempty"`
	Paths             JSON     `json:"paths,omitempty"`
	Webhooks          JSON     `json:"webhooks,omitempty"`
	Components        JSON     `json:"components,omitempty"`
	Security          []JSON   `json:"security,omitempty"`
	Tags              []JSON   `json:"tags,omitempty"`
	ExternalDocs      JSON     `json:"externalDocs,omitempty"`
}

// Info follows the spec from https://swagger.io/specification/#info-object
type Info struct {
	Title          string   `json:"title"`
	Summary        string   `json:"summary,omitempty"`
	Description    string   `json:"description,omitempty"`
	TermsOfService string   `json:"termsOfService,omitempty"`
	Contact        *Contact `json:"contact,omitempty"`
	License        *License `json:"license,omitempty"`
	Version        string   `json:"version"`
}

// Contact follows the spec from https://swagger.io/specification/#contact-object
type Contact struct {
	Name  string `json:"name,omitempty"`
	Url   string `json:"url,omitempty"`
	Email string `json:"email,omitempty"`
}

// License follows the spec from https://swagger.io/specification/#license-object
type License struct {
	Name       string `json:"name"`
	Identifier string `json:"identifier,omitempty"`
	Url        string `json:"url,omitempty"`
}

// Server follows the spec from https://swagger.io/specification/#server-object
type Server struct {
	Url         string              `json:"url"`
	Description string              `json:"description,omitempty"`
	Variables   map[string]Variable `json:"variables,omitempty"`
}

// Variable follows the spec from https://swagger.io/specification/#server-variable-object
type Variable struct {
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default"`
	Description string   `json:"description"`
}
