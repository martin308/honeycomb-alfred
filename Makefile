all: package

package: build
	zip -r honeycomb-alfred.alfredworkflow honeycomb-alfred info.plist

build:
	go build .

