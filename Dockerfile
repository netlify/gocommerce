FROM golang:1.11

RUN useradd -m netlify

ADD . /src
RUN cd /src && make deps build_linux && mv gocommerce /usr/local/bin/

USER netlify
CMD ["gocommerce"]
EXPOSE 8080
