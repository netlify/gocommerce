FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/gocommerce

RUN cd /go/src/github.com/netlify/gocommerce && make deps build && mv gocommerce /usr/local/bin/

CMD ["gocommerce"]
