#!/bin/bash -e

DEBUG=$(echo $DEBUG | tr '[:upper:]' '[:lower:]')
[ "${DEBUG}" == "true" ] && set -x

which greadlink >/dev/null 2>&1 && rlink='greadlink' || rlink='readlink'
my_file="$($rlink -e "$0")"
my_dir="$(dirname $my_file)"

cd $my_dir/../..

res=0

function run_make() {
    local target=$1
    printf "INFO: run make $target\n\n"
    if ! make $target ; then
        printf "\nERROR: make $target failed\n\n"
        res=1
    else
        printf "\nINFO: make $target passed\n\n"
    fi
}

# TODO: uncomment and fix if required
# run_make generate

run_make lint

run_make "modules verify-modules"

printf "INFO: run go test\n\n"
if ! go test -race ./... ; then
    printf "\nERROR: go test failed\n\n"
    res=1
else
    printf "\nINFO: go test passed\n\n"
fi

exit $res
