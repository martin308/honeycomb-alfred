all: package

package: build
	zip -r honeycomb-alfred.alfredworkflow \
		honeycomb-alfred \
		update-available.png \
		icon.png \
		info.plist

build:
	go build .

