package numbers

func TryUpcast(n interface{}) interface{} {
	switch nn := n.(type) {
	case int8:
		return int64(nn)
	case int16:
		return int64(nn)
	case int32:
		return int64(nn)
	case int:
		return int64(nn)
	case int64:
		return nn
	case float32:
		return float64(nn)
	case float64:
		return nn
	default:
		return n
	}
}
