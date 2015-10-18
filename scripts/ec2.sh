#!/bin/bash
# Ugly script to install dependencies and start drender on ec2.
#
# Prerequisites:
# * drender binary in current dir
# * qpov.conf in current dir, containing export statements for AWS auth.
#

if [ "$2" = "" ]; then
    echo "Usage: $0 <host> <sshkey>"
    exit 1
fi

set -e
HOST="$1"
SSHKEY="$2"
EC2USER=ubuntu
BIN=drender

DEST="${EC2USER}@${HOST}"

echo "Installing dependencies..."
ssh -oStrictHostKeyChecking=no -i "${SSHKEY}" "${DEST}" "echo deb http://us-east-1.ec2.archive.ubuntu.com/ubuntu/ trusty-backports main restricted universe multiverse | sudo tee /etc/apt/sources.list.d/multi.list > /dev/null && echo deb http://us-east-1.ec2.archive.ubuntu.com/ubuntu/ trusty main multiverse | sudo tee -a /etc/apt/sources.list.d/multi.list && sudo apt-get -y update && sudo apt-get install -y screen povray schedtool wget atop rar curl vim"

echo "Copying files..."
scp -oStrictHostKeyChecking=no -i "${SSHKEY}" "${HOME}/.screenrc" drender qpov.conf "${DEST}:"

echo "Starting screen..."
ssh -oStrictHostKeyChecking=no -i "${SSHKEY}" "${DEST}" "screen -dmS qpov"

echo "Creating directories..."
ssh -oStrictHostKeyChecking=no -i "${SSHKEY}" "${DEST}" "mkdir -p qpov"

echo "Starting qpov..."
ssh -oStrictHostKeyChecking=no -i "${SSHKEY}" "${DEST}" 'screen -x qpov -X stuff ". ./qpov.conf && ./drender -queue=qpov -wd=qpov\\n"'
