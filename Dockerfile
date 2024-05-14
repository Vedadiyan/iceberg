FROM alpine 
COPY iceberg /srv
RUN cd /srv
RUN chmod 777 iceberg
WORKDIR /srv
CMD ./iceberg