#!/bin/sh

# DLV="/home/vmc/go/bin/dlv"
DLV="$GOPATH/bin/dlv"

if [ "$DEBUG_AS_ROOT" = "true" ]; then
	echo Run as Root
	exec sudo -C 4 -E env "PATH=$PATH" "$DLV" --only-same-user=false "$@"
else
	echo Run as User
	exec "$DLV" "$@"
fi

