QPov
====

Copyright Thomas Habets <thomas@habets.se> 2015

https://github.com/ThomasHabets/qpov

Introduction
============
QPov converts Quake demos into POV-Ray files, ready to render.
[Example video](https://www.youtube.com/watch?v=jzcevsd5SGE).

Installing
==========
```
$ mkdir go
$ cd go
$ GOPATH=$(pwd) go get github.com/ThomasHabets/qpov
$ GOPATH=$(pwd) go build github.com/ThomasHabets/qpov/cmd/{bsp,dem,mdl,pak,render}
```

You'll need the Quake data files, either shareware or full version.

Optionally you can also use the
[Quake Reforged](http://quakeone.com/reforged/downloads.html)
replacement textures

Running
=======
You need to convert Quake maps and models in addition to the demos.

```
$ bsp /.../pak0.pak convert -out demo1
$ mdl /.../pak0.pak convert -out demo1
$ mkdir demo1
$ dem /.../pak0.pak convert -out demo1 -fps 30 -camera_light demo1.dem
$ render -fast demo1/*.pov
```

Hacking
=======

Example frames
--------------
Assuming 30fps and QDQr recam:
* light levels:
  * e1m1: 10 610
* water:
  * e1m1: 232 610
  * e1m3: 300
