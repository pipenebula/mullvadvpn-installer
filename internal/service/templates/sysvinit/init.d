#!/bin/sh
### BEGIN INIT INFO
# Provides:          mullvad-daemon
# Required-Start:    $network $syslog
# Required-Stop:     $network $syslog
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Mullvad VPN daemon
### END INIT INFO

DAEMON=/usr/bin/mullvad-daemon
OPTS="-v --disable-stdout-timestamps"
NAME=mullvad-daemon
PIDFILE=/var/run/$NAME.pid
LOGTAG=$NAME

start() {
  logger -t "$LOGTAG" "starting"
  start-stop-daemon --start --quiet --make-pidfile --pidfile $PIDFILE \
    --exec $DAEMON -- $OPTS
}

stop() {
  logger -t "$LOGTAG" "stopping"
  start-stop-daemon --stop --quiet --pidfile $PIDFILE
}

case "$1" in
  start)   start ;;
  stop)    stop ;;
  restart) stop; start ;;
  status)  status_of_proc -p $PIDFILE $DAEMON && exit 0 || exit $? ;;
  *)       echo "Usage: $0 {start|stop|restart|status}"; exit 1 ;;
esac

exit 0
