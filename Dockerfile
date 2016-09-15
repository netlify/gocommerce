FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/gocommerce

RUN cd /go/src/github.com/netlify/gocommerce && glide install && go build -o /usr/local/bin/gocommerce

CMD ["gocommerce"]
