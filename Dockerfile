# Start from the official Golang image
FROM golang:1.21.1-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN go build -o main .

# Expose port 3000
EXPOSE 3000

# Run the application
CMD ["./main"]