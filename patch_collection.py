import re

with open("collectionv4/collection.go", "r") as f:
    orig = f.read()

# Replace Collection struct
struct_target = """type Collection struct {
name     string
filepath atomic.Pointer[string]
store    Store
records  records.Records[Record]
maxID    atomic.Int64
count    atomic.Int64
autoID   atomic.Int64
indexes  atomic.Pointer[map[string]Index]
defaults atomic.Pointer[map[string]any]
writerMu sync.Mutex
}"""
struct_repl = """type Collection struct {
name     string
filepath atomic.Pointer[string]
store    Store
records  records.Records[Record]
maxID    atomic.Int64
count    atomic.Int64
autoID   atomic.Int64
indexes  atomic.Pointer[map[string]Index]
defaults atomic.Pointer[map[string]any]
writerMu sync.Mutex
idxReqs  chan asyncIndexReq
idxDone  chan struct{}
}

type asyncIndexReq struct {
op      uint8
id      int64
data    []byte
oldData []byte
done    chan struct{}
}"""

orig = orig.replace(struct_target, struct_repl)

# Replace NewCollection
newcol_target = """records.NewRecordsUltra[Record](),
}"""
newcol_repl = """records.NewRecordsUltra[Record](),
s: make(chan asyncIndexReq, 1000000),
e: make(chan struct{}),
}"""
orig = orig.replace(newcol_target, newcol_repl)

# Replace NewCollection return
newcol_ret_target = """c.defaults.Store(&emptyDefaults)
return c
}"""

newcol_ret_repl = """c.defaults.Store(&emptyDefaults)
go c.indexWorker()
return c
}

func (c *Collection) indexWorker() {
for req := range c.idxReqs {
req.done != nil {
.done)
tinue
dexes := *c.indexes.Load()

req.op {
OpInsert:
_, idx := range indexes {
!idx.IsUnique() {
= idx.Add(req.id, req.data)
OpDelete:
_, idx := range indexes {
!idx.IsUnique() {
= idx.Remove(req.id, req.oldData)
OpUpdate:
_, idx := range indexes {
!idx.IsUnique() {
= idx.Remove(req.id, req.oldData)
= idx.Add(req.id, req.data)
e)
}

func (c *Collection) SyncIndexes() {
done := make(chan struct{})
c.idxReqs <- asyncIndexReq{done: done}
<-done
}

func (c *Collection) asyncIndexOp(op uint8, id int64, data []byte, oldData []byte) {
var dataCopy []byte
if data != nil {
 = append([]byte(nil), data...)
}
var oldDataCopy []byte
if oldData != nil {
 = append([]byte(nil), oldData...)
}
c.idxReqs <- asyncIndexReq{
     op,
     id,
   dataCopy,
oldDataCopy,
}
}
"""
orig = orig.replace(newcol_ret_target, newcol_ret_repl)

# Replace Close
close_target = """func (c *Collection) Close() error {
if c.store == nil {
 nil
}
return c.store.Close()
}"""
close_repl = """func (c *Collection) Close() error {
if c.idxReqs != nil {
s)
e
}
if c.store == nil {
 nil
}
return c.store.Close()
}"""
orig = orig.replace(close_target, close_repl)

# Replace Insert index operations
insert_target = """indexes := *c.indexes.Load()
asyncIndexes, err := indexInsert(indexes, id, jsonData)
if err != nil {
t.Add(-1)
 0, err
}

// 2. Escribir en el Journal
if err := c.store.Append(OpInsert, id, jsonData, wait); err != nil {
Rollback si falla el journal
dexes := *c.indexes.Load()
dexRemove(indexes, id, jsonData)
t.Add(-1)
 0, fmt.Errorf("journal write failed: %v", err)
}

for _, index := range asyncIndexes {
func(idx Index, i int64, d []byte) {
= idx.Add(i, d)
dex, id, append([]byte(nil), jsonData...))
}

return id, nil"""
insert_repl = """indexes := *c.indexes.Load()
err := indexInsertSync(indexes, id, jsonData)
if err != nil {
t.Add(-1)
 0, err
}

// 2. Escribir en el Journal
if err := c.store.Append(OpInsert, id, jsonData, wait); err != nil {
Rollback si falla el journal
dexes := *c.indexes.Load()
dexRemoveSync(indexes, id, jsonData)
t.Add(-1)
 0, fmt.Errorf("journal write failed: %v", err)
}

c.asyncIndexOp(OpInsert, id, jsonData, nil)

return id, nil"""
if insert_target in orig:
orig = orig.replace(insert_target, insert_repl)
else:
print("insert_target NOT FOUND")

# Replace Delete index operations
del_target = """indexes := *c.indexes.Load()
err := indexRemove(indexes, id, rec.Data)
if err != nil {
 fmt.Errorf("could not free index: %w", err)
}

// Persistir el borrado (payload vacío)
if err := c.store.Append(OpDelete, id, nil, wait); err != nil {
Si el log falla, tenemos que deshacer el indexRemove, pero es complejo.
Al menos devolvemos error
 err
}

// Liberar memoria para el GC y marcar como inactivo
c.records.Delete(id)
c.count.Add(-1)

return nil"""
del_repl = """indexes := *c.indexes.Load()
err := indexRemoveSync(indexes, id, rec.Data)
if err != nil {
 fmt.Errorf("could not free index: %w", err)
}

// Persistir el borrado (payload vacío)
if err := c.store.Append(OpDelete, id, nil, wait); err != nil {
Si el log falla, tenemos que deshacer el indexRemove, pero es complejo.
Al menos devolvemos error
 err
}

c.asyncIndexOp(OpDelete, id, nil, rec.Data)

// Liberar memoria para el GC y marcar como inactivo
c.records.Delete(id)
c.count.Add(-1)

return nil"""
orig = orig.replace(del_target, del_repl)

# Recover updates
orig = orig.replace("indexRemove(*c.indexes.Load(), id, rec.Data)", "indexRemoveFull(*c.indexes.Load(), id, rec.Data)")
orig = orig.replace("asyncIdx, _ := indexInsert(*c.indexes.Load(), id, data)\nfor _, idx := range asyncIdx {\n_ = idx.Add(id, data)\n}", "indexInsertFull(*c.indexes.Load(), id, data)")

with open("collectionv4/collection.go", "w") as f:
    f.write(orig)
