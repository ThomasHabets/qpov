// -*- html -*-
package main

const joinTmpl = `
{{ $root := . }}
<h2>On Ubuntu/Debian</h2>
<pre>
# Its runs in the foreground, so you may want to do "screen -S qpov" first.
sudo apt-get install povray povray-includes schedtool
wget -O drender https://cdn.habets.se/tmp/drender-$(uname -m)
chmod +x drender
mkdir wd
./drender -scheduler=qpov.retrofitta.se:9999 -wd=wd
</pre>
<h2>Docker image</h2>
Can be found <a href="https://hub.docker.com/r/thomashabets/qpov/">here</a>.
`
