#!/bin/bash

COLLECTIONS_URL=https://inceptiondb.io/collections

curl $COLLECTIONS_URL -d '{
  "name": "pokemon"
}'

curl $COLLECTIONS_URL/pokemon/indexes -d '{"field":"num"}'

curl $COLLECTIONS_URL/pokemon -d @pokemon.jsonl

