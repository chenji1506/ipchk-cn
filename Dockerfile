FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY ipw-server /app/main
COPY setting.json /app/
RUN adduser -D appuser && chown -R appuser:appuser /app
USER appuser
EXPOSE 8080
CMD ["/app/main"]
