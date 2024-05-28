FROM golang:alpine

COPY . /app
WORKDIR /app

RUN go build

EXPOSE 8122

CMD ["./logcollector"]