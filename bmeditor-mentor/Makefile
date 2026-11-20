# Builds the extension into dist/, the folder BMEditor installs: the
# manifest, the frontend script, and every program under cmd/ for each
# system in TARGETS. Needs Node, and Go when there is a cmd/. BMEditor runs
# `make dist` itself when it installs the extension from its git repository.
# bmx dev builds only for your system (TARGETS); bmx pack makes the zip.

TARGETS ?= linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64
FILES   := README.md CHANGELOG.md LICENSE icon.svg

.PHONY: dist clean

dist:
	rm -rf dist
	$(if $(wildcard package-lock.json),npm ci,npm install) --no-audit --no-fund
	npm run build
	@for c in $(wildcard cmd/*); do for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		echo "go build $$c $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" -o dist/bin/$${c#cmd/}-$$os-$$arch$$ext ./$$c || exit 1; \
	done; done
	cp -r $(wildcard $(FILES)) dist/
	@# dist is installed as it is: its manifest has no build step.
	node -e "const m = require('./bmext.json'); delete m.build; require('fs').writeFileSync('dist/bmext.json', JSON.stringify(m, null, 2) + '\\n')"

clean:
	rm -rf dist node_modules *.zip
