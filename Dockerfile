FROM golang:1.17 AS build

RUN useradd -m netlify

WORKDIR /src
COPY . .

# Build the application
RUN make deps build_linux
RUN mv gocommerce /usr/local/bin/

# Stage 2: Create a minimal image
FROM scratch

# Copy the built binary from the first stage
COPY --from=build /usr/local/bin/gocommerce /usr/local/bin/gocommerce

USER netlify
EXPOSE 8080

CMD ["gocommerce"]

