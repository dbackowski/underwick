#!/bin/sh
# Builds the browser version and publishes it to the gh-pages branch, which GitHub Pages serves.
#
# The build embeds the Oryx sprites, so it happens here, where assets/ exists; CI never has them. Each
# deploy replaces gh-pages with a single fresh commit, so 28MB builds don't pile up in the repo's history.
set -eu
cd "$(dirname "$0")/.."

if [ ! -d assets/Character ]; then
	echo "assets/ is missing: copy the Oryx sprites in first, see README" >&2
	exit 1
fi

GOOS=js GOARCH=wasm go build -o web/underwick.wasm .
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/

version=$(git describe --always --dirty)
remote=$(git remote get-url origin)
site=$(mktemp -d)
trap 'rm -rf "$site"' EXIT
cp web/index.html web/wasm_exec.js web/underwick.wasm "$site"
touch "$site/.nojekyll" # serve the files as they are

cd "$site"
git init -q -b gh-pages
git add .
git commit -q -m "Deploy $version"
git push -q -f "$remote" gh-pages
echo "Deployed $version to gh-pages."
