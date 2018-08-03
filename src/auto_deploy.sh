#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd $DIR
git checkout master > /dev/null 2>&1
git pull > /dev/null 2>&1

LAST_GIT_SHA="$(cat $DIR/bin/static/git_sha.txt)"
CURRENT_GIT_SHA="$(git rev-parse HEAD)"

# echo Last: $LAST_GIT_SHA
# echo Current: $CURRENT_GIT_SHA

if [ "$LAST_GIT_SHA" != "$CURRENT_GIT_SHA" ]
then
  echo "$(date '+%Y %b %d %H:%M') Start deploy. ($LAST_GIT_SHA -> $CURRENT_GIT_SHA)"
  $DIR/deploy.sh
  echo "$(date '+%Y %b %d %H:%M') Finished deploy ($CURRENT_GIT_SHA)"
fi
