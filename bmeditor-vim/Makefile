# Builds the installable extension into dist/. `make zip` packages it;
# CI publishes the zip as GitHub releases (scripts/publish.sh).

VERSION := $(shell sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' bmext.json)

.PHONY: dist zip clean

dist:
	rm -rf dist
	npm ci --no-audit --no-fund
	npm run build
	cp README.md CHANGELOG.md LICENSE icon.svg dist/
	@# The built extension is installed as it is: its manifest has no build step.
	node -e "const m = require('./bmext.json'); delete m.build; require('fs').writeFileSync('dist/bmext.json', JSON.stringify(m, null, 2) + '\\n')"

zip: dist
	rm -f vim-$(VERSION).zip
	cd dist && zip -qr ../vim-$(VERSION).zip .
	@echo "vim-$(VERSION).zip"

clean:
	rm -rf dist node_modules vim-*.zip
