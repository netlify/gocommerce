FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/commerce

RUN cd /go/src/github.com/netlify/commerce && make deps build && mv commerce /usr/local/bin/

CMD ["commerce"]
