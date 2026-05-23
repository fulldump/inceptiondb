package fson

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/valyala/fastjson"
)

type ObjectJSON struct {
	Keys   []string
	Values []any
}

func (o *ObjectJSON) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	// TIP PRO: Usar UseNumber() es vital en bases de datos.
	// Evita que Go convierta números grandes (ej: IDs tipo int64) en float64 perdiendo precisión.
	dec.UseNumber()

	// 1. Esperar el inicio del objeto '{'
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("se esperaba '{', se obtuvo %v", t)
	}

	// 2. Pre-asignar capacidad si es posible, o resetear si reutilizamos el struct
	o.Keys = o.Keys[:0]
	o.Values = o.Values[:0]

	// 3. Leer el flujo de tokens (Streaming)
	for dec.More() {
		// Leer la Llave
		t, err := dec.Token()
		if err != nil {
			return err
		}

		key, ok := t.(string)
		if !ok {
			return fmt.Errorf("se esperaba llave de tipo string, se obtuvo %T", t)
		}

		// Leer el Valor
		// Usamos Decode porque el valor puede ser un primitivo ("hola")
		// o una estructura compleja anidada ([1, 2, 3]).
		var val any
		if err := dec.Decode(&val); err != nil {
			return err
		}

		// Llenado paralelo de la estructura SoA (Structure of Arrays)
		o.Keys = append(o.Keys, key)
		o.Values = append(o.Values, val)
	}

	// 4. Consumir el token de cierre '}'
	_, err = dec.Token()
	return err
}

func (o *ObjectJSON) UnmarshalJSON_V2(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))

	// Esperar inicio de objeto '{'
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("se esperaba '{', se obtuvo %v", t)
	}

	// Resetear slices manteniendo la capacidad si existe
	o.Keys = o.Keys[:0]
	o.Values = o.Values[:0]

	for dec.More() {
		// 1. Leer Llave
		t, err := dec.Token()
		if err != nil {
			return err
		}
		key, ok := t.(string)
		if !ok {
			return fmt.Errorf("se esperaba llave string, se obtuvo %T", t)
		}

		// 2. Leer Valor
		var val any
		if err := dec.Decode(&val); err != nil {
			return err
		}

		// 3. Llenado paralelo
		o.Keys = append(o.Keys, key)
		o.Values = append(o.Values, val)
	}

	// Consumir '}'
	_, err = dec.Token()
	return err
}

func (o *ObjectJSON) Get(key string) any {
	for i, k := range o.Keys {
		if k == key {
			return o.Values[i]
		}
	}
	return nil
}

var parserPool fastjson.ParserPool

func FlattenJSON(data []byte) (*ObjectJSON, error) {
	// Obtenemos un parser del pool para evitar alocaciones constantes
	p := parserPool.Get()
	defer parserPool.Put(p)

	v, err := p.ParseBytes(data)
	if err != nil {
		return nil, err
	}

	// Obtenemos el objeto raíz
	obj, err := v.Object()
	if err != nil {
		return nil, err
	}

	// Pre-alocamos el slice con la cantidad exacta de llaves
	result := &ObjectJSON{
		Keys:   make([]string, 0, obj.Len()),
		Values: make([]any, 0, obj.Len()),
	}

	// Visitamos cada par llave-valor
	obj.Visit(func(key []byte, v *fastjson.Value) {
		var val any

		// Mapeo simple de tipos fastjson -> Go
		switch v.Type() {
		case fastjson.TypeString:
			val = string(v.GetStringBytes())
		case fastjson.TypeNumber:
			val = v.GetFloat64()
		case fastjson.TypeTrue:
			val = true
		case fastjson.TypeFalse:
			val = false
		case fastjson.TypeNull:
			val = nil
		case fastjson.TypeObject:
			// Podríamos recurrir recursivamente si quisiéramos objetos anidados
			val = v.String()
		case fastjson.TypeArray:
			val = v.String()
		}

		result.Keys = append(result.Keys, string(key))
		result.Values = append(result.Values, val)
	})

	return result, nil
}
