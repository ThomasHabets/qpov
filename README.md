# QPov

Copyright Thomas Habets <thomas@habets.se> 2015-2016

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

```shell
mkdir go
cd go
GOPATH=$(pwd) go get github.com/ThomasHabets/qpov
for bin in bsp dem mdl pak render; do GOPATH=$(pwd) go build github.com/ThomasHabets/qpov/cmd/$bin;done
```

You'll need the Quake data files, either shareware or full version.

Optionally you can also use the
[Quake Reforged](http://quakeone.com/reforged/downloads.html)
replacement textures.

## Running
You need to convert Quake maps and models in addition to the demos.

```shell
mkdir demo1
bsp -pak /.../pak0.pak convert -lights=false -out demo1
mdl -pak /.../pak0.pak convert -out demo1
dem -pak /.../pak0.pak convert -out demo1 -fps 30 -camera_light=true demo1.dem
render -fast demo1/*.pov
avconv -r 30 -i demo1/frame-%08d.png -f mp4 -q:v 0 -vcodec mpeg4 demo1.mp4
```

To mix in audio (can be created using `sound.sh` in `demo1` directory), run:
```shell
avconv -i demo1.mp4 -i sound.wav -c copy demo1-sound.mp4
```

### Running a render node

Suitable for EC2 Ubuntu:
```shell
sed -i 's/^# deb / deb /' /etc/apt/sources.list
apt-get update
apt-get install schedtool povray{,-includes} screen rar
mkdir qpov-wd
drender -scheduler=addr-of-dscheduler:12345 -wd=qpov-wd
```

### Scheduling render work

```shell
dmaster \
    -scheduler addr-of-dscheduler:12345 \
    -package https://foo/bar.tar.gz \
    -dir balcony \
    -file balcony.pov \
    +Q11 +A0.3 +R4 +W3840 +H2160
```

### Using retextures

```shell
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

## General tips

* When experimenting with light settings (`-light_fade_distance` and `-light_fade_power`)
  it's useful to add options `+RP5 +RS9` to hop around a bit as the frame is rendering.

### Interesting links

* https://www.quaddicted.com/engines/software_vs_glquake
