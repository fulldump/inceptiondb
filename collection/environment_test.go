package collection

import (
	"fmt"
	"os"
	"time"
)

func Environment(f func(filename string)) {
	filename := fmt.Sprintf("temp-%v", time.Now().UnixNano())
	defer os.Remove(filename)

	f(filename)
}
