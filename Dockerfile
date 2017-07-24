FROM netlify/go-glide:v0.12.3

ADD . /go/src/github.com/netlify/gocommerce

RUN useradd -m netlify && cd /go/src/github.com/netlify/gocommerce && make deps build_linux && mv gocommerce /usr/local/bin/

USER netlify
CMD ["gocommerce"]
EXPOSE 8080
