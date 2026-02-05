FROM alpine:latest

RUN apk --no-cache add ca-certificates sqlite

WORKDIR /root/

# Copy the pre-built binary
COPY mdp-ir-car-parking .

# Make it executable
RUN chmod +x mdp-ir-car-parking

# Expose port
EXPOSE 8080

CMD ["./mdp-ir-car-parking"]
