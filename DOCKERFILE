FROM alpine 
COPY iceberg /srv
WORKDIR /srv
RUN chmod 777 ./iceberg
CMD ./iceberg