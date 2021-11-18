package box

type Attr map[string]interface{}

const (
	AttrHttpMethod = "attr_http_method"
	AttrHttpBind   = "attr_http_bind"
	AttrDoc        = "attr_doc"
)

// Get Attribute value using key string from Box, Resource or Action.
func (a Attr) GetAttribute(key string) (value interface{}) {
	value, exists := a[key]

	if !exists {
		return nil
	}

	return
}

// Set Attribute key-value to Box, Resource or Action.
func (a Attr) SetAttribute(key string, value interface{}) {
	a[key] = value
}
