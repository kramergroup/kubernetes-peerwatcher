FROM iron/base
WORKDIR /app

# copy binary into image
COPY bin/peerwatcher /app/
ENTRYPOINT ["./peerwatcher"]
