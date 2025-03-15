FROM golang:1.24.1 AS builder

WORKDIR /app

COPY . .

RUN make build

FROM scratch

COPY --from=builder /app/bin/inceptiondb /inceptiondb

ENV HTTPADDR=:8080

CMD ["/inceptiondb"]
