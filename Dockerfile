FROM alpine:3.12.1

COPY ./easy-ig /app/easy-ig

RUN chmod +x /app/easy-ig

EXPOSE 8211

CMD ["/app/easy-ig"]