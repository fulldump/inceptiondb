# InceptionDB Backlog

## Must have

* Filter collection by perfect match subdocument
* Fullscan with offset/skip and limit
* Authentication and logical collection groups (databases)
* Index management in UI

## Should have

* Filter collection by expressions (similar to connor library)
* Quotas
  * Max memory: per collection, also per database?
  * Max disk: per collection, also per database?
  * Max rows: per collection, also per database?
  * Max document size: per collection and per database
  * Max collections per database
* Default primary key on field _id (require non sparse indexes)

## Could have

* Implement not sparse indexes
* Filter by JS function
* Patch by JS function
* Support UTF-8 in collection names
* Composed indexes (key is made by multiple fields combined)
* Automatic _id index can have multiple value sources (configured at collection level)
  * uuid
  * autoincrement
  * unixnano
* Ensure thread safety and improve performance with Map from standard library

## Won't have

* Disk sync indexes
* Instantaneous consistency
