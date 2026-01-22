VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

.PHONY: all build build-cross clean test package

all: test build

build:
	go build ${LDFLAGS} -o kamstrup2mqtt

build-cross:
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o kamstrup2mqtt-amd64
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o kamstrup2mqtt-arm64

test:
	go test -v ./...

clean:
	rm -f kamstrup2mqtt kamstrup2mqtt-amd64 kamstrup2mqtt-arm64
	rm -rf dist

package: build-cross deb
	mkdir -p dist/kamstrup2mqtt
	cp kamstrup2mqtt-amd64 dist/kamstrup2mqtt/
	cp kamstrup2mqtt-arm64 dist/kamstrup2mqtt/
	cp packaging/install.sh dist/kamstrup2mqtt/
	cp packaging/config.json.example dist/kamstrup2mqtt/
	cp packaging/kamstrup2mqtt.service dist/kamstrup2mqtt/
	chmod +x dist/kamstrup2mqtt/install.sh
	cd dist && tar -czvf kamstrup2mqtt-$(VERSION).tar.gz kamstrup2mqtt
	echo "Tarball package created: dist/kamstrup2mqtt-$(VERSION).tar.gz"

deb: build-cross
	@echo "Building Debian packages..."
	@for arch in amd64 arm64; do \
		DEB_DIR=dist/deb_$$arch; \
		rm -rf $$DEB_DIR; \
		mkdir -p $$DEB_DIR/DEBIAN; \
		mkdir -p $$DEB_DIR/usr/bin; \
		mkdir -p $$DEB_DIR/etc/kamstrup2mqtt; \
		mkdir -p $$DEB_DIR/lib/systemd/system; \
		mkdir -p $$DEB_DIR/usr/share/doc/kamstrup2mqtt; \
		\
		cp kamstrup2mqtt-$$arch $$DEB_DIR/usr/bin/kamstrup2mqtt; \
		cp packaging/config.json.example $$DEB_DIR/etc/kamstrup2mqtt/config.json; \
		cp packaging/kamstrup2mqtt.service $$DEB_DIR/lib/systemd/system/; \
		cp packaging/config.json.example $$DEB_DIR/usr/share/doc/kamstrup2mqtt/; \
		\
		cp packaging/debian/conffiles $$DEB_DIR/DEBIAN/; \
		cp packaging/debian/postinst $$DEB_DIR/DEBIAN/; \
		cp packaging/debian/prerm $$DEB_DIR/DEBIAN/; \
		cp packaging/debian/postrm $$DEB_DIR/DEBIAN/; \
		cp packaging/debian/templates $$DEB_DIR/DEBIAN/; \
		cp packaging/debian/config $$DEB_DIR/DEBIAN/; \
		chmod +x $$DEB_DIR/DEBIAN/postinst $$DEB_DIR/DEBIAN/prerm $$DEB_DIR/DEBIAN/postrm $$DEB_DIR/DEBIAN/config; \
		\
		sed "s/\$${VERSION}/$(VERSION)/g; s/\$${ARCH}/$$arch/g" packaging/debian/control > $$DEB_DIR/DEBIAN/control; \
		\
		dpkg-deb --build $$DEB_DIR dist/kamstrup2mqtt_$(VERSION)_$$arch.deb; \
		rm -rf $$DEB_DIR; \
	done
