#!/bin/sh
# Deterministic scanner stand-in for browser smoke tests.
case " $* " in
  *" 198.51.100.99 "*)
    trap 'exit 0' TERM INT
    while :; do sleep 1; done
    ;;
  *" -sn "*)
    printf '%s\n' '<nmaprun><host><status state="up"/><address addr="192.0.2.20" addrtype="ipv4"/></host></nmaprun>'
    exit 0
    ;;
  *" -sV "*)
    printf '%s\n' '<nmaprun><host><address addr="192.0.2.20" addrtype="ipv4"/><ports><port protocol="tcp" portid="80"><state state="open"/><service name="http" product="nginx" version="1.24"/></port></ports></host></nmaprun>'
    exit 0
    ;;
  *" --script "*)
    printf '%s\n' '<nmaprun><host><ports><port><script id="http-title" output="AnchorScan test"/></port></ports></host></nmaprun>'
    exit 0
    ;;
  *" -json -status-code "*)
    printf '%s\n' '{"url":"http://192.0.2.20:80","status-code":200,"title":"AnchorScan","tech":["nginx"]}'
    exit 0
    ;;
  *" -target "*)
    printf '%s\n' '{"template-id":"anchorscan-test","matched-at":"http://192.0.2.20:80","info":{"name":"AnchorScan test","severity":"info"}}'
    exit 0
    ;;
esac
printf '%s\n' '[80]'
