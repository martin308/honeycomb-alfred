all: package

package: build
	zip -r honeycomb-alfred.alfredworkflow \
		honeycomb-alfred \
		update-available.png \
		icon.png \
		info.plist

build: clean
	go build .

clean:
	rm -f *.alfredworkflow && rm -f honeycomb-alfred
