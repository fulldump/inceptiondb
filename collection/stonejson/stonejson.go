package stonejson

import (
	"fmt"
	"unsafe"

	"github.com/buger/jsonparser"
)

type ValueCoord struct {
	Offset uint32
	Length uint32
	Type   uint8 // Representa String, Number, Bool, etc.
}

type ObjectJSON struct {
	Data   []byte       // Referencia al JSON original (el "bloque de piedra")
	Keys   []string     // Nombres de las llaves
	Coords []ValueCoord // Coordenadas de los valores en el bloque Data
}

// Definimos tipos internos para evitar lógica pesada
const (
	TypeUnknown uint8 = iota
	TypeNull
	TypeNumber
	TypeString
	TypeBool
	TypeObject
	TypeArray
)

func (o *ObjectJSON) Get(key string) any {
	for i, k := range o.Keys {
		if k == key {
			coord := o.Coords[i]
			// Solo aquí, en el momento que el usuario pide el dato,
			// extraemos el valor del bloque original.
			raw := o.Data[coord.Offset : coord.Offset+coord.Length]

			return o.cast(raw, coord.Type)
		}
	}
	return nil
}

// cast convierte el pedazo de bytes al tipo Go correspondiente "bajo demanda"
func (o *ObjectJSON) cast(raw []byte, t uint8) any {
	switch t {
	case TypeString:
		// Quitamos las comillas si es un string crudo del JSON
		if len(raw) >= 2 && raw[0] == '"' {
			return string(raw[1 : len(raw)-1])
		}
		return string(raw)
	case TypeNumber:
		// Aquí puedes usar strconv.ParseFloat o json.Number
		return string(raw)
	case TypeBool:
		return raw[0] == 't' // 't' de true
	default:
		return raw
	}
}

// ParseToOffsets escanea el JSON y anota dónde está cada valor sin hacer copias
func ParseToOffsets(data []byte) (*ObjectJSON, error) {
	obj := &ObjectJSON{
		Data: data,
		// Pre-asignamos una capacidad razonable para evitar re-alojamientos de memoria
		Keys:   make([]string, 0, 10),
		Coords: make([]ValueCoord, 0, 10),
	}

	// ObjectEach itera solo por el primer nivel del JSON (no entra en objetos anidados).
	// Es extremadamente rápido porque solo busca comas y dos puntos.
	err := jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {

		// 1. Mapear el tipo de dato de la librería a nuestro byte interno
		var t uint8
		switch dataType {
		case jsonparser.String:
			t = TypeString
		case jsonparser.Number:
			t = TypeNumber
		case jsonparser.Boolean:
			t = TypeBool
		case jsonparser.Null:
			t = TypeNull
		case jsonparser.Object:
			t = TypeObject
		case jsonparser.Array:
			t = TypeArray
		default:
			t = TypeUnknown
		}

		// 2. El truco de magia negra (pero seguro en Go):
		// Como 'value' es un sub-slice que apunta al array original 'data',
		// podemos calcular el offset restando sus direcciones de memoria.
		startOffset := uint32(uintptr(unsafe.Pointer(&value[0])) - uintptr(unsafe.Pointer(&data[0])))

		// 3. Guardar las coordenadas
		obj.Keys = append(obj.Keys, string(key)) // Aquí hay una pequeña alocación por el string de la llave
		obj.Coords = append(obj.Coords, ValueCoord{
			Offset: startOffset,
			Length: uint32(len(value)),
			Type:   t,
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error parseando offsets del JSON: %v", err)
	}

	return obj, nil
}
