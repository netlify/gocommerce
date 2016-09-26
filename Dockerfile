FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/netlify-commerce

RUN useradd -m netlify && cd /go/src/github.com/netlify/netlify-commerce && make deps build && mv netlify-commerce /usr/local/bin/

USER netlify
CMD ["netlify-commerce"]
