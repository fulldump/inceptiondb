# InceptionDB

A home made database based on journal to store JSON documents.


## Motivation

InceptionDB was born to be the persistence layer of another project called Bitumen. Bitumen is a distributed NAS that should store family memories, pictures and videos that should last for 50 or 100 years.

One of the biggest motivations is the code should be simple and easy to maintain (yes, even the database itself) and also 100% free of license restrictions.

The second biggest motivation is to have fun and learn some things :D.

## Technical overview

InceptionDB stores all the data in memory and also a copy on disk in the form of a journal.

When the service starts, the journal is read and applied to recreate the last valid state in memory. From that point on, it is ready to continue operation. One lateral effect is that you can recover the state of the whole database in any point in the past.

It support unique indexes that can be:
* `sparse` value can be undefined (so, document is not indexed and reachable by the index)
* `multivaluated` multiple values will point to the same record (if the value is an array of strings)

B+ index is a MUST before using it on a real application.

It does not have a scheduler, so the user has the responsibility to choose the proper index (or do a fullscan). The main drawback is adding a new index will require to modify the application.


## Performance

InceptionDB has been designed to be small and easy to read, not to be blazing fast but some performance tests reach more than 200K inserts per second with 2 indexes in one node.


## Features

* API oriented - HTTP is the only interface so that it can be used by any language with any technology.
* Based on journal
* Fast writes


## Future work

There are some features planned for the future: replication, trigger http events, atomic patch defined by javascript, historical data,... 

## Getting started

Just clone the repo, execute `make run` (golang is required) and open (http://127.0.0.1:8080/)[http://127.0.0.1:8080/].

![image](https://user-images.githubusercontent.com/2371070/193629843-a8f6e66b-a97d-48e4-9c0b-33478eeb909c.png)


Choose a good name:

![image](https://user-images.githubusercontent.com/2371070/193629504-f3a9a3b7-fc3e-43a4-ad78-ec9e042873c7.png)

And insert your first JSON:

![image](https://user-images.githubusercontent.com/2371070/193629672-45cb4871-8321-43b8-8667-01e02f9445dd.png)




