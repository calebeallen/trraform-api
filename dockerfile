FROM golang:1.24.0
    
WORKDIR /app

COPY . .

RUN go build -o main .

CMD ["/app/main"]