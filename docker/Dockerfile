# sudo docker build -t qpov .
# sudo docker images | grep qpov
# sudo docker tag -f ID_HERE thomashabets/qpov:latest
# sudo docker push thomashabets/qpov
FROM ubuntu:15.04
MAINTAINER thomas@habets.se
RUN echo deb http://archive.ubuntu.com/ubuntu/ vivid main multiverse > /etc/apt/sources.list.d/multi.list
RUN apt-get -y update
RUN apt-get install -y povray schedtool wget atop rar curl vim
COPY drender /
EXPOSE 4900
CMD /drender -queue=qpov -wd=/tmp -addr=:4900
