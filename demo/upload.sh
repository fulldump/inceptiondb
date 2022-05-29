#!/bin/bash

curl https://inceptiondb.io/collections -d '{
  "name": "pokemon"
}'

curl https://inceptiondb.io/collections/pokemon -d @pokemon.jsonl

