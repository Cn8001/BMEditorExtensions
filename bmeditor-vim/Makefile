# Builds the installable extension into dist/: the manifest, the frontend
# script, and vim-host, which reads the vimrc, for each system in TARGETS.
# Needs Node and Go. bmx dev builds it for your system only; bmx pack makes
# the zip, which CI publishes as GitHub releases (scripts/publish.sh).

TARGETS ?= linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64

.PHONY: dist clean

dist:
	rm -rf dist
	npm ci --no-audit --no-fund
	npm run build
	@for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "go build vim-host $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" -o dist/bin/vim-host-$$os-$$arch$$ext ./cmd/vim-host || exit 1; \
	done
	cp README.md CHANGELOG.md LICENSE icon.svg dist/
	@# The built extension is installed as it is: its manifest has no build step.
	node -e "const m = require('./bmext.json'); delete m.build; require('fs').writeFileSync('dist/bmext.json', JSON.stringify(m, null, 2) + '\\n')"

clean:
	rm -rf dist node_modules vim-*.zip
