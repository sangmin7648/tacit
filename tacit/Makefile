.PHONY: build clean whisper-lib test e2e-test install rg-download app

WHISPER_DIR  := core/internal/components/transcriber/whisper.cpp
WHISPER_BUILD := $(WHISPER_DIR)/build

# Stacit libraries produced by whisper.cpp cmake build
WHISPER_LIBS := \
	$(WHISPER_BUILD)/src/libwhisper.a \
	$(WHISPER_BUILD)/ggml/src/libggml.a \
	$(WHISPER_BUILD)/ggml/src/libggml-base.a \
	$(WHISPER_BUILD)/ggml/src/libggml-cpu.a

# The oldest macOS the binaries run on. Without a target, everything is built
# for the build machine's own macOS, and the result refuses to start on
# anything older.
#
# whisper.cpp's build directory is cached: after changing this, run
# `rm -rf $(WHISPER_BUILD)` once so its libraries are rebuilt for the new target.
MACOS_MIN := 13.0

# Platform-specific libraries
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
	WHISPER_LIBS += $(WHISPER_BUILD)/ggml/src/ggml-metal/libggml-metal.a
	WHISPER_LIBS += $(WHISPER_BUILD)/ggml/src/ggml-blas/libggml-blas.a
	PLATFORM_LDFLAGS := -lstdc++ \
		-framework Accelerate \
		-framework Foundation \
		-framework Metal \
		-framework MetalKit
	MIN_FLAG := -mmacosx-version-min=$(MACOS_MIN)
	WHISPER_CMAKE_FLAGS := -DCMAKE_OSX_DEPLOYMENT_TARGET=$(MACOS_MIN)
else
	PLATFORM_LDFLAGS := -lstdc++ -lm -lpthread
endif

# CGo flags (passed via environment)
export CGO_CFLAGS  := -I$(abspath $(WHISPER_DIR)/include) -I$(abspath $(WHISPER_DIR)/ggml/include) -O2 $(MIN_FLAG)
export CGO_LDFLAGS := $(foreach lib,$(WHISPER_LIBS),$(abspath $(lib))) $(PLATFORM_LDFLAGS) $(MIN_FLAG)

# VERSION is stamped into both binaries (main.version) and the app's
# Info.plist. The release workflow passes the tag; a local build describes the
# checkout, e.g. v0.11.0-16-gfe768d8.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GO_LDFLAGS := -X main.version=$(VERSION)
# CFBundleShortVersionString takes numbers only: v0.11.0-16-gfe768d8 -> 0.11.0.
PLIST_VERSION := $(or $(shell echo '$(VERSION)' | sed -nE 's/^v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/p'),0.0.0)

# ── Targets ──────────────────────────────────────────────

TEN_VAD_FRAMEWORK := core/internal/components/speech-detector/ten-vad/lib/macOS/ten_vad.framework

RG_VERSION := 14.1.1
RG_ARM64_URL := https://github.com/BurntSushi/ripgrep/releases/download/$(RG_VERSION)/ripgrep-$(RG_VERSION)-aarch64-apple-darwin.tar.gz
RG_AMD64_URL := https://github.com/BurntSushi/ripgrep/releases/download/$(RG_VERSION)/ripgrep-$(RG_VERSION)-x86_64-apple-darwin.tar.gz

core/internal/components/note-manager/rg-darwin-arm64:
	@echo "Downloading ripgrep $(RG_VERSION) for darwin/arm64..."
	@tmpdir=$$(mktemp -d) && \
	  curl -fsSL $(RG_ARM64_URL) | tar -xz -C $$tmpdir && \
	  mv $$tmpdir/ripgrep-$(RG_VERSION)-aarch64-apple-darwin/rg $@ && \
	  rm -rf $$tmpdir
	@echo "Downloaded: $@"

core/internal/components/note-manager/rg-darwin-amd64:
	@echo "Downloading ripgrep $(RG_VERSION) for darwin/amd64..."
	@tmpdir=$$(mktemp -d) && \
	  curl -fsSL $(RG_AMD64_URL) | tar -xz -C $$tmpdir && \
	  mv $$tmpdir/ripgrep-$(RG_VERSION)-x86_64-apple-darwin/rg $@ && \
	  rm -rf $$tmpdir
	@echo "Downloaded: $@"

rg-download: core/internal/components/note-manager/rg-darwin-arm64 core/internal/components/note-manager/rg-darwin-amd64

build: whisper-lib rg-download
	go build -ldflags "$(GO_LDFLAGS)" -o tacit ./app/cli/
ifeq ($(UNAME_S),Darwin)
	@echo "Bundling ten_vad.framework..."
	rm -rf ten_vad.framework
	cp -R $(TEN_VAD_FRAMEWORK) ten_vad.framework
	install_name_tool -rpath "$$(otool -l tacit | grep -A2 LC_RPATH | grep path | awk '{print $$2}')" @executable_path tacit
endif

# PKGS and ARGS narrow a run while keeping the CGo flags above, which a bare
# `go test` of a whisper-linked package lacks:
#   make test PKGS=./core/workflows/listen/ ARGS='-run TestRunSource -count=1'
test: whisper-lib
	go test $(or $(PKGS),./...) $(ARGS)

whisper-lib: $(WHISPER_BUILD)/src/libwhisper.a

$(WHISPER_BUILD)/src/libwhisper.a:
	@command -v cmake >/dev/null 2>&1 || { echo "Error: cmake is required. Install with: brew install cmake"; exit 1; }
	@echo "Building whisper.cpp stacit libraries..."
	cmake -B $(WHISPER_BUILD) -S $(WHISPER_DIR) \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DWHISPER_BUILD_SERVER=OFF \
		-DCMAKE_BUILD_TYPE=Release \
		$(WHISPER_CMAKE_FLAGS)
	cmake --build $(WHISPER_BUILD) --config Release -j

e2e-test: build
	go test -tags e2e -v -count=1 ./core/workflows/listen/ -run TestE2E
	go test -tags integration -v -count=1 ./core/internal/components/note-classifier/ -run TestClassifier
	go test -tags integration -v -count=1 ./core/workflows/listen/ -run TestClassifier

# ── Mac app ──────────────────────────────────────────────
#
# Tacit.app is the menu-bar front end (app/desktop) with the CLI bundled
# beside it, which it runs as `tacit listen`:
#
#   Contents/MacOS/Tacit                  menu-bar app (pure Go + Wails), with
#                                         the onboarding window's frontend
#                                         (app/desktop/frontend, Svelte)
#                                         embedded; building it needs Node
#   Contents/Helpers/tacit                the CLI from `make build`
#   Contents/Frameworks/ten_vad.framework
#
# The CLI is in Helpers, not beside the app in MacOS: the default macOS
# filesystem is case-insensitive, so MacOS/tacit would overwrite MacOS/Tacit.
# The framework lives in Frameworks, where codesign expects nested code, so the
# bundled CLI gets an extra rpath to find it there.
#
# Signing is ad-hoc by default. macOS keys the microphone grant to an ad-hoc
# build's exact hash, so every rebuild asks again; sign with
# a stable identity (e.g. a self-signed "Code Signing" certificate made in
# Keychain Access) to keep grants across rebuilds:
#
#   make app SIGN_IDENTITY="Tacit Dev"
#
# Symptom of an ad-hoc rebuild: System Settings shows Tacit allowed for the
# microphone, yet the daemon it starts hears nothing — the grant belongs to an
# earlier build. Reset it and grant again:
#
#   tccutil reset Microphone io.github.sangmin7648.tacit

APP := build/Tacit.app
FRONTEND := app/desktop/frontend
SIGN_IDENTITY ?= -

# CGO_* are overridden for the app: the whisper link flags exported above are the
# CLI's, and the app links none of it.
app: build
	cd $(FRONTEND) && npm ci --no-audit --no-fund && npm run build
	rm -rf $(APP)
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Helpers $(APP)/Contents/Frameworks
	CGO_CFLAGS="$(MIN_FLAG)" CGO_LDFLAGS="$(MIN_FLAG)" \
		go build -ldflags "$(GO_LDFLAGS)" -o $(APP)/Contents/MacOS/Tacit ./app/desktop/
	cp tacit $(APP)/Contents/Helpers/tacit
	install_name_tool -add_rpath @executable_path/../Frameworks $(APP)/Contents/Helpers/tacit
	cp -R ten_vad.framework $(APP)/Contents/Frameworks/
	cp app/desktop/Info.plist $(APP)/Contents/Info.plist
	plutil -replace CFBundleShortVersionString -string "$(PLIST_VERSION)" $(APP)/Contents/Info.plist
	plutil -replace CFBundleVersion -string "$(PLIST_VERSION)" $(APP)/Contents/Info.plist
	codesign --force --sign "$(SIGN_IDENTITY)" $(APP)/Contents/Frameworks/ten_vad.framework
	codesign --force --sign "$(SIGN_IDENTITY)" $(APP)/Contents/Helpers/tacit
	codesign --force --sign "$(SIGN_IDENTITY)" $(APP)
	codesign --verify --strict --deep $(APP)
	@echo "Built $(APP)"

INSTALL_DIR := $(HOME)/.local/bin

install: build
	@mkdir -p $(INSTALL_DIR)
	cp tacit $(INSTALL_DIR)/tacit-dev
	chmod +x $(INSTALL_DIR)/tacit-dev
ifeq ($(UNAME_S),Darwin)
	rm -rf $(INSTALL_DIR)/ten_vad.framework
	cp -R ten_vad.framework $(INSTALL_DIR)/ten_vad.framework
	xattr -dr com.apple.quarantine $(INSTALL_DIR)/tacit-dev 2>/dev/null || true
	xattr -dr com.apple.quarantine $(INSTALL_DIR)/ten_vad.framework 2>/dev/null || true
	codesign --force --deep --sign - $(INSTALL_DIR)/tacit-dev 2>/dev/null || true
	codesign --force --deep --sign - $(INSTALL_DIR)/ten_vad.framework 2>/dev/null || true
endif
	@echo "Installed to $(INSTALL_DIR)/tacit-dev"

clean:
	rm -rf $(WHISPER_BUILD)
	rm -rf build
	rm -rf ten_vad.framework
	rm -f tacit
	rm -f core/internal/components/note-manager/rg-darwin-arm64 core/internal/components/note-manager/rg-darwin-amd64
	go clean
