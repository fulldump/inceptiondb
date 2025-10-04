# InceptionDB Backlog

## Must have

* ~~Filter collection by perfect match subdocument~~
* ~~Fullscan with offset/skip and limit~~
* ~~Authentication and logical collection groups (databases)~~
* Index management in UI
* BUGFIX: when document is modified to remove field from non-sparse index, it should NOT remove the field!!!
* ~~BTree index~~

## Should have

* ~~Filter collection by expressions (similar to connor library)~~
* Quotas
  * Max memory: per collection, also per database?
  * Max disk: per collection, also per database?
  * Max rows: per collection, also per database?
  * Max document size: per collection and per database
  * Max collections per database
* Default primary key on field _id (require non sparse indexes)
* ~~UI should be usable on mobile devices (left panel should not be always present)~~

## Could have

* ~~Implement not sparse indexes~~
* Filter by JS function
* Patch by JS function
* ~~Support UTF-8 in collection names~~
* ~~Compound indexes (key is made by multiple fields combined)~~
* Automatic _id index can have multiple value sources (configured at collection level)
  * uuid
  * autoincrement
  * unixnano
* Ensure thread safety and improve performance with Map from standard library
* ~~Insert multiple documents per request~~
* Return http.StatusServiceUnavailable while loading collections

## Won't have

* Disk sync indexes
* Instantaneous consistency



## Optimizations
* collection insert: calculating defaults only when needed: speedup 10.16%
