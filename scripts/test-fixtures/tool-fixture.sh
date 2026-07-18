#!/bin/sh
# Deterministic scanner stand-in for browser smoke tests.
case " $* " in
  *" 198.51.100.99 "*)
    trap 'exit 0' TERM INT
    while :; do sleep 1; done
    ;;
esac
printf '%s\n' '[80]'
