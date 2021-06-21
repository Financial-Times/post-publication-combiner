FROM peteclarkft/ersatz:0.0.4

# Kafka, __internal-content-api, __document-store-api and utility (__gtg, __health) endpoints mocking
COPY ./ersatz-fixtures.yml /_ft/ersatz-fixtures.yml

WORKDIR /
CMD [ "/ersatz", "-p", "8080" ]

