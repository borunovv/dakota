#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR
git checkout master
git pull

rm -rf $DIR/bin
mkdir $DIR/bin

go build -o $DIR/bin/dakota-server $DIR/dakota-server.go

cp -R $DIR/static $DIR/bin/
git rev-parse HEAD > $DIR/bin/static/git_sha.txt

cp -R $DIR/shell/*.sh $DIR/bin/

#stop server
if [ -f "/srv/dakota/bin/stop.sh" ]
then
	/srv/dakota/bin/stop.sh
fi
	
rm -rf /srv/dakota/bin
cp -R $DIR/bin /srv/dakota

/srv/dakota/bin/start.sh

