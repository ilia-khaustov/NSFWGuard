FROM golang

RUN apt-get update && apt-get install -y inotify-tools dumb-init

ENV PKG NSFWGuard

COPY ./sbin/pkg-watch.sh /sbin/pkg-watch
RUN chmod +x /sbin/pkg-watch

WORKDIR "$GOPATH/src/$PKG"
VOLUME "$GOPATH/src/$PKG"

ENTRYPOINT ["/usr/bin/dumb-init", "--"]
CMD ["/sbin/pkg-watch", "go", "run", "run_tgbot.go"]