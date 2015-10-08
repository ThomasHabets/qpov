# QPov

Copyright Thomas Habets <thomas@habets.se> 2015

https://github.com/ThomasHabets/qpov

## Introduction
QPov takes the data files from Quake and turns them into POV-Ray,
which is a raytracer that then renders them to png.
[Example video](https://www.youtube.com/watch?v=jzcevsd5SGE).

## Why
Modern CPUs are idle too much. Ray tracing tends to be able to
consume any and CPU cycles you give to it. QPOV generates data
that POV-Ray can then use to solve this problem.

A second reason is that Quake is getting harder to get running
in good quality. GLQuake from Steam doesn't start on my machine,
and if I want to watch Quake Done Quick then all the videos
on YouTube are either low res, or has other settings that make
them crap.

I also want to experiment with POV-Ray and for example make
programatic realistic water, and a quake demo gives me a nice 3D
animation to work with.

## Installing
```
$ mkdir go
$ cd go
$ GOPATH=$(pwd) go get github.com/ThomasHabets/qpov
$ GOPATH=$(pwd) go build github.com/ThomasHabets/qpov/cmd/{bsp,dem,mdl,pak,render}
```

You'll need the Quake data files, either shareware or full version.

Optionally you can also use the
[Quake Reforged](http://quakeone.com/reforged/downloads.html)
replacement textures.

## Running
You need to convert Quake maps and models in addition to the demos.

```
$ mkdir demo1
$ bsp -pak /.../pak0.pak convert -out demo1
$ mdl -pak /.../pak0.pak convert -out demo1
$ dem -pak /.../pak0.pak convert -out demo1 -fps 30 -camera_light demo1.dem
$ render -fast demo1/*.pov
```

### Running a render node.

```
export AWS_ACCESS_KEY_ID=…
export AWS_SECRET_ACCESS_KEY=…
apt-get install schedtool povray{,-includes}
mkdir qpov-wd
drender -queue myqueue -wd qpov-wd
```

### Scheduling render work

```
export AWS_ACCESS_KEY_ID=…
export AWS_SECRET_ACCESS_KEY=…
dmaster \
    -queue myqueue \
    -package s3://mybucket/balcony.rar \
    -dir balcony \
    -file balcony.pov \
    -destination s3://mybucket/balcony/ \
    +Q11 +A0.3 +R4 +W3840 +H2160
```

### Using retextures
```
7zr x QRP_map_textures_v.1.00.pk3.7z
unzip QRP_map_textures_v.1.00.pk3
for d in . *; do (cd $d && for i in *.tga; do convert $i $(basename $i .tga).png; done); done
```

And then use `-retexture=/path/to/textures` with `bsp`.

## Hacking

### Example frames
Assuming 30fps and QDQr recam:
* light levels:
  * e1m1: 10 610
* brown slime:
  * e1m1: 232 610
  * e1m3: 300

### Blog posts this project
* https://blog.habets.se/2015/03/Raytracing-Quake-demos

### Interesting links
* https://www.quaddicted.com/engines/software_vs_glquake
