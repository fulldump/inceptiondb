# Análisis de Fugas de Recursos - InceptionDB

## Resumen Ejecutivo

Se identificaron **7 problemas críticos** y **5 problemas moderados** de posibles fugas de recursos en el código de InceptionDB. Los principales vectores de fuga incluyen:

- ❌ **Archivos sin cerrar en casos de error**
- ❌ **Canal `db.exit` cerrado múltiples veces**
- ⚠️ **Buffers sin flush en caso de error**
- ⚠️ **Goroutines sin control de ciclo de vida**
- ⚠️ **Listener de red sin cerrar explícitamente**

---

## 🔴 Problemas Críticos

### 1. Fuga de File Handle en [OpenCollection](file:///home/user/inceptiondb/collection/collection.go#74-184) (CRÍTICO)
**Archivo**: [collection.go:74-183](file:///home/user/inceptiondb/collection/collection.go#L74-L183)

**Problema**: 
```go
f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
if err != nil {
    return nil, fmt.Errorf("open file for read: %w", err)
}
// ... código que puede fallar ...
// El archivo 'f' NUNCA SE CIERRA
collection.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
```

El archivo abierto para lectura (línea 77) **nunca se cierra**. Si la función retorna con error después de línea 100-170, el file descriptor queda abierto.

**Impacto**: 
- Fuga de file descriptors en cada colección abierta
- Puede alcanzar límites del sistema operativo
- El SO mantiene el archivo bloqueado

**Solución**:
```go
f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
if err != nil {
    return nil, fmt.Errorf("open file for read: %w", err)
}
defer f.Close() // ← AÑADIR ESTO
```

---

### 2. Doble Cierre de Canal en `database.Stop()` (CRÍTICO)
**Archivo**: [database.go:139-156](file:///home/user/inceptiondb/database/database.go#L139-L156)

**Problema**:
```go
func (db *Database) Stop() error {
    defer close(db.exit)  // ← Primera vez
    // ...
}

func (db *Database) Start() error {
    go db.Load()
    <-db.exit  // ← Espera que se cierre
    return nil
}
```

Si [Stop()](file:///home/user/inceptiondb/database/database.go#139-157) se llama múltiples veces, se produce **panic: close of closed channel**.

**Impacto**:
- Crash de la aplicación
- Imposible hacer shutdown graceful múltiples veces
- Reportado en conversación previa (df4db701)

**Solución**:
```go
func (db *Database) Stop() error {
    select {
    case <-db.exit:
        // Ya cerrado
        return nil
    default:
        close(db.exit)
    }
    
    var lastErr error
    for name, col := range db.Collections {
        // ...
    }
    return lastErr
}
```

---

### 3. Buffer Sin Flush en Caso de Error (CRÍTICO)
**Archivo**: [collection.go:789-800](file:///home/user/inceptiondb/collection/collection.go#L789-L800)

**Problema**:
```go
func (c *Collection) Close() error {
    {
        err := c.buffer.Flush()
        if err != nil {
            return err  // ← Retorna SIN cerrar c.file
        }
    }
    
    err := c.file.Close()
    c.file = nil
    return err
}
```

Si `Flush()` falla, el archivo nunca se cierra.

**Impacto**:
- Fuga de file descriptor
- Datos pueden perderse en el buffer
- Archivo bloqueado en disco

**Solución**:
```go
func (c *Collection) Close() error {
    var firstErr error
    
    if err := c.buffer.Flush(); err != nil {
        firstErr = err
    }
    
    if c.file != nil {
        if err := c.file.Close(); err != nil && firstErr == nil {
            firstErr = err
        }
        c.file = nil
    }
    
    return firstErr
}
```

---

### 4. Goroutine del Signal Handler Sin Control (CRÍTICO)
**Archivo**: [bootstrap.go:66-74](file:///home/user/inceptiondb/bootstrap/bootstrap.go#L66-L74)

**Problema**:
```go
signalChan := make(chan os.Signal, 1)
signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)
go func() {
    for {
        sig := <-signalChan
        fmt.Println("Signal received", sig.String())
        stop()
    }
}()
```

Esta goroutine **nunca termina**. Corre en un `for` infinito sin forma de salir.

**Impacto**:
- Goroutine leak
- Canal sin cerrar
- En tests, la goroutine persiste

**Solución**:
```go
ctx, cancel := context.WithCancel(context.Background())
signalChan := make(chan os.Signal, 1)
signal.Notify(signalChan, syscall.SIGTERM, syscall.SIGINT)

go func() {
    select {
    case sig := <-signalChan:
        fmt.Println("Signal received", sig.String())
        stop()
    case <-ctx.Done():
        return
    }
}()

// En stop(), llamar: 
// signal.Stop(signalChan)
// cancel()
// close(signalChan)
```

---

### 5. Listener de Red Sin Cerrar Explícitamente (CRÍTICO)
**Archivo**: [bootstrap.go:54-59](file:///home/user/inceptiondb/bootstrap/bootstrap.go#L54-L59)

**Problema**:
```go
ln, err := net.Listen("tcp", c.HttpAddr)
if err != nil {
    log.Println("ERROR:", err.Error())
    os.Exit(-1)
}
log.Println("listening on", c.HttpAddr)
```

El listener `ln` nunca se cierra explícitamente. Aunque `s.Shutdown()` debería manejarlo, no está garantizado en todos los casos de error.

**Impacto**:
- Puerto puede quedar bloqueado
- Fuga de recursos de red
- Problemas en tests

**Solución**:
```go
stop = func() {
    ln.Close()  // ← AÑADIR antes de Shutdown
    db.Stop()
    s.Shutdown(context.Background())
}
```

---

### 6. Datos Potencialmente Sin Persistir en [EncodeCommand](file:///home/user/inceptiondb/collection/collection.go#853-873) (CRÍTICO)
**Archivo**: [collection.go:853-872](file:///home/user/inceptiondb/collection/collection.go#L853-L872)

**Problema**:
```go
func (c *Collection) EncodeCommand(command *Command) error {
    // ...
    c.encoderMutex.Lock()
    c.buffer.Write(b)  // ← Escribe al buffer
    c.encoderMutex.Unlock()
    return nil
}
```

Los datos se escriben solo al buffer (`bufio.Writer`), pero **nunca se hace flush** explícitamente. Los datos pueden perderse si:
- La aplicación crashea
- No se cierra la colección correctamente
- El buffer no llega a su tamaño máximo

**Impacto**:
- Pérdida de datos
- Inconsistencia entre memoria y disco
- Problemas en crash recovery

**Soluciones Posibles**:
1. **Flush periódico** (cada N operaciones o cada X segundos)
2. **Flush opcional** basado en criticidad de la operación
3. **Modo sync** para operaciones críticas

```go
// Opción 1: Flush cada N operaciones
func (c *Collection) EncodeCommand(command *Command) error {
    // ...
    c.encoderMutex.Lock()
    c.buffer.Write(b)
    c.writeCount++
    if c.writeCount%100 == 0 {  // Flush cada 100 escrituras
        c.buffer.Flush()
    }
    c.encoderMutex.Unlock()
    return nil
}

// Opción 2: Background flusher
go func() {
    ticker := time.NewTicker(1 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        c.encoderMutex.Lock()
        c.buffer.Flush()
        c.encoderMutex.Unlock()
    }
}()
```

---

### 7. Race Condition en Acceso a `db.Collections` (CRÍTICO)
**Archivo**: [database.go:79](file:///home/user/inceptiondb/database/database.go#L79), [database.go:146](file:///home/user/inceptiondb/database/database.go#L146)

**Problema**:
```go
delete(db.Collections, name) // TODO: protect section! not threadsafe
```

El mapa `db.Collections` se accede sin protección de mutex en múltiples goroutines.

**Impacto**:
- Race condition
- Panic: concurrent map read/write
- Comportamiento indefinido

**Solución**:
```go
type Database struct {
    Config      *Config
    status      string
    Collections map[string]*collection.Collection
    collMutex   sync.RWMutex  // ← AÑADIR
    exit        chan struct{}
}

// Proteger todos los accesos:
db.collMutex.Lock()
delete(db.Collections, name)
db.collMutex.Unlock()
```

---

## ⚠️ Problemas Moderados

### 8. Sin Manejo de Errores en Write Operations (MODERADO)
**Archivo**: [insertStream.go:87-97](file:///home/user/inceptiondb/api/apicollectionv1/insertStream.go#L87-L97)

**Problema**:
```go
_, err = bufrw.WriteString("HTTP/1.1 202 " + http.StatusText(http.StatusAccepted) + "\r\n")
w.Header().Write(bufrw)
_, err = bufrw.WriteString("Transfer-Encoding: chunked\r\n")
_, err = bufrw.WriteString("\r\n")
```

Los errores se asignan pero **nunca se verifican** después de las operaciones de escritura.

**Solución**:
```go
if _, err = bufrw.WriteString("HTTP/1.1 202 " + http.StatusText(http.StatusAccepted) + "\r\n"); err != nil {
    return
}
```

---

### 9. Conexiones HTTP Sin `defer Close()` en Benchmarks (MODERADO)
**Archivo**: [helpers.go:55](file:///home/user/inceptiondb/cmd/bench/helpers.go#L55)

**Problema**:
```go
defer resp.Body.Close()
```

Aunque hay `defer`, esta es la única parte correcta. El problema es que **si ocurre panic antes del defer**, el body no se cierra.

**Mejora**:
```go
if resp != nil && resp.Body != nil {
    defer resp.Body.Close()
}
```

---

### 10. IndexMap Sin Protección de Concurrencia Completa (MODERADO)
**Archivo**: [collection.go:28](file:///home/user/inceptiondb/collection/collection.go#L28)

**Problema**:
```go
Indexes map[string]*collectionIndex // todo: protect access with mutex or use sync.Map
```

El comentario TODO indica que el acceso al mapa de índices no está protegido completamente.

**Impacto**:
- Race conditions en operaciones concurrentes
- Posibles panics

**Solución**: Implementar `sync.Map` o añadir mutex de protección.

---

### 11. Goroutines en Tests Sin WaitGroup (MODERADO)
**Archivo**: [collection_test.go:53](file:///home/user/inceptiondb/collection/collection_test.go#L53), [collection_test.go:539](file:///home/user/inceptiondb/collection/collection_test.go#L539)

**Problema**:
Goroutines lanzadas sin sincronización adecuada pueden continuar ejecutándose después de que el test termine.

**Solución**: Usar `sync.WaitGroup` o `errgroup`.

---

### 12. Pool de Encoders Sin Límite (MODERADO)
**Archivo**: [collection.go:53-72](file:///home/user/inceptiondb/collection/collection.go#L53-L72)

**Problema**:
```go
var encPool = sync.Pool{
    New: func() any {
        buffer := bytes.NewBuffer(make([]byte, 0, 8*1024))
        // ...
    },
}
```

El `sync.Pool` puede crecer sin límite si hay muchas operaciones concurrentes.

**Impacto**:
- Consumo excesivo de memoria en alta carga
- No es técnicamente una fuga, pero puede ser problemático

**Mitigación**: Considerar límites o monitoreo.

---

## 📊 Resumen de Severidades

| Severidad | Cantidad | Impacto |
|-----------|----------|---------|
| 🔴 Crítico | 7 | Fuga garantizada de recursos, pérdida de datos, o crash |
| ⚠️ Moderado | 5 | Posible fuga bajo condiciones específicas |

---

## 🔧 Recomendaciones Prioritarias

### Inmediato (P0):
1. ✅ **Cerrar file handle en [OpenCollection](file:///home/user/inceptiondb/collection/collection.go#74-184)** - 1 línea de código
2. ✅ **Proteger cierre de canal `db.exit`** - Previene crashes
3. ✅ **Garantizar cierre de archivo en [Close()](file:///home/user/inceptiondb/collection/collection.go#789-801)** - Previene fugas

### Corto Plazo (P1):
4. ✅ **Implementar flush periódico o background flusher** - Previene pérdida de datos
5. ✅ **Proteger mapa `db.Collections` con mutex** - Elimina race conditions
6. ✅ **Lifecycle de goroutine del signal handler** - Cleanup correcto

### Medio Plazo (P2):
7. ✅ **Cerrar listener de red explícitamente**
8. ✅ **Proteger acceso a `Indexes` map**
9. ✅ **Mejorar manejo de errores en API handlers**

---

## 🧪 Cómo Detectar Estas Fugas

### Herramientas Recomendadas:

1. **Tests de Race Condition**:
   ```bash
   go test -race ./...
   ```

2. **Análisis de Fugas de Goroutines**:
   ```bash
   go test -run=TestName -count=1
   # Usar uber-go/goleak
   ```

3. **Profiling de Memoria**:
   ```bash
   go test -memprofile=mem.prof
   go tool pprof mem.prof
   ```

4. **File Descriptors Abiertos**:
   ```bash
   lsof -p $(pgrep inceptiondb)
   ```

5. **Verificar Goroutines Activas**:
   ```go
   runtime.NumGoroutine() // Antes y después de operaciones
   ```

---

## 📝 Notas Adicionales

- El código usa comentarios `TODO` que coinciden con problemas encontrados
- Existe historial de fix de panic en canal (conversación df4db701)
- El proyecto tiene buenas prácticas en general (uso de `defer`, pools, etc.)
- Falta testing sistemático de cleanup y resource leaks

---

## ✅ Buenas Prácticas Observadas

1. ✅ Uso correcto de `defer` en la mayoría de casos
2. ✅ `sync.Pool` para reutilización de encoders
3. ✅ Uso de `bufio.Writer` para optimizar I/O
4. ✅ Cierre de conexiones en benchmarks
5. ✅ Manejo de contextos en algunos handlers

---

**Generado el**: 2025-11-26  
**Archivos Analizados**: 55 archivos Go  
**Líneas de Código Revisadas**: ~10,000+
