# SYSTEM.md: Arquitectura del Motor de Base de Datos In-Memory

Este documento detalla las especificaciones técnicas, las decisiones de diseño y el razonamiento arquitectónico detrás del motor de base de datos de alto rendimiento desarrollado en Go.

## 1. Requisitos del Sistema

El motor ha sido diseñado bajo los siguientes pilares de ingeniería:

* **Latencia Ultra-baja:** Operaciones de lectura y escritura en el rango de nanosegundos (capacidad medida de >3.5M ops/seg).
* **Persistencia Garantizada:** Tolerancia a fallos mediante un registro de operaciones tipo WAL (Write-Ahead Log).
* **Esquema Flexible:** Almacenamiento de documentos JSON de tamaño variable (desde pocos bytes hasta varios MB).
* **Eficiencia de Memoria:** Minimización del impacto del Garbage Collector (GC) de Go al manejar volúmenes de 10M+ de registros.
* **Recuperación Rápida:** Reconstrucción total del estado de la base de datos en segundos mediante escaneo secuencial.

---

## 2. Decisiones de Diseño y Alternativas

### A. Estructura de Datos: Flat Slice (Index Array)
La memoria principal se organiza como un `slice` contiguo de estructuras fijas donde el índice del slice actúa como el identificador interno (ID).

* **Decisión:** Usar `[]Record` donde cada `Record` contiene el payload y metadatos de estado.
* **Alternativas:**
    * *Hash Maps:* Búsqueda $O(1)$ pero con alto overhead de hashing y presión sobre el GC.
    * *B-Trees:* Eficientes para rangos, pero lentos para acceso por ID directo debido a la navegación por nodos $O(\log n)$.
* **Razón:** El **Flat Slice** aprovecha la **localidad de caché del CPU**. Al ser memoria contigua, el hardware realiza *prefetching* de datos, eliminando latencias de acceso a RAM.



### B. Gestión de Huecos: FreeList (Pila de Reutilización)
Para evitar el desplazamiento de elementos en eliminaciones ($O(n)$), se utiliza una estrategia de gestión de espacios vacíos.

* **Decisión:** Una pila (Stack) LIFO que almacena los IDs de registros eliminados.
* **Alternativas:**
    * *Compactación en caliente:* Mover datos para rellenar huecos. Descartado por ser extremadamente costoso y por invalidar punteros.
    * *Bitmap de disponibilidad:* Un array de bits para marcar vacíos. Descartado porque requiere un escaneo $O(n)$ para encontrar el próximo hueco libre.
* **Razón:** La **FreeList** permite que tanto la inserción como el borrado sean $O(1)$ constantes y garantiza que el tamaño del slice no crezca indefinidamente si hay rotación de datos.

### C. Persistencia: Write-Ahead Log (WAL) con CRC32
Cada operación se persiste en un archivo binario *append-only* antes de ser confirmada en la estructura de memoria.

* **Decisión:** Formato binario con un header fijo de 17 bytes: `[OpCode(1)][ID(8)][Length(4)][CRC32(4)]`.
* **Alternativas:**
    * *JSON/CSV:* Descartados por lentitud de parseo y verbosidad.
    * *MD5/SHA:* Descartados por alto consumo de CPU.
* **Razón:** El **CRC32** (específicamente la tabla Castagnoli) cuenta con aceleración por hardware en CPUs modernos (SSE4.2). Proporciona integridad contra corrupciones de disco con un impacto casi nulo en el rendimiento.



### D. Durabilidad: Estrategia de Sync Diferido
Para maximizar el throughput, se separa la escritura en el buffer del sistema operativo de la escritura física en el plato del disco.

* **Decisión:** Uso de `bufio.Writer` con un proceso de `fsync` (Sync) en segundo plano (ej. cada 500ms).
* **Alternativas:**
    * *Synchronous I/O:* Ejecutar `Sync()` en cada insert. Garantiza integridad total pero limita el motor a la velocidad del disco duro (IOPS bajos).
* **Razón:** El balance entre rendimiento (millones de ops/seg) y seguridad. El riesgo de pérdida se acota a una ventana temporal mínima (el intervalo del flusher).

### E. Manejo de Payloads Variables (JSON)
Dado que los JSON pueden variar drásticamente de tamaño, se tratan como bloques de memoria dinámicos.

* **Decisión:** El `FlatSlice` almacena el `[]byte` (puntero y longitud) del documento.
* **Alternativas:**
    * *Fixed-size Slots:* Dividir la memoria en bloques fijos. Descartado por la fragmentación interna masiva con documentos JSON.
* **Razón:** Go gestiona eficientemente los slices de bytes. Al mantener el índice (`FlatSlice`) separado de los datos, las operaciones de escaneo y mantenimiento de la base de datos no necesitan tocar los payloads pesados.

---

## 3. Flujo de Recuperación (Recovery)

Al arrancar, el motor reconstruye su estado siguiendo estos pasos:

1.  **Replay:** Se lee el WAL secuencialmente.
2.  **Integridad:** Se verifica el CRC32 de cada registro. Si falla, se asume corrupción y se detiene la carga para proteger la base de datos.
3.  **Mapping:**
    * `OpInsert/Update`: Se coloca el payload en el `FlatSlice[ID]`.
    * `OpDelete`: Se limpia la posición `FlatSlice[ID]` y se marca como inactiva.
4.  **Reconstrucción de FreeList:** Se realiza un escaneo final del slice para identificar huecos y poblar la pila de IDs disponibles para nuevas inserciones.

---

## 4. Rendimiento Observado (Benchmark)

Basado en pruebas con 10 millones de documentos:

| Operación | Rendimiento | Tasa de Transferencia |
| :--- | :--- | :--- |
| **Inserción** | ~3.6M docs/seg | ~360 MB/s |
| **Recuperación** | ~4.9M docs/seg | ~490 MB/s |

