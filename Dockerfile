FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/gocommerce

RUN useradd -m netlify && cd /go/src/github.com/netlify/gocommerce && make deps build && mv gocommerce /usr/local/bin/

USER netlify
CMD ["gocommerce"]
