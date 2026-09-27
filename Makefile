.PHONY: build clean whisper-lib test e2e-test install rg-download app

WHISPER_DIR  := third_party/whisper.cpp
WHISPER_BUILD := $(WHISPER_DIR)/build

# Stacit libraries produced by whisper.cpp cmake build
WHISPER_LIBS := \
	$(WHISPER_BUILD)/src/libwhisper.a \
	$(WHISPER_BUILD)/ggml/src/libggml.a \
	$(WHISPER_BUILD)/ggml/src/libggml-base.a \
	$(WHISPER_BUILD)/ggml/src/libggml-cpu.a

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
else
	PLATFORM_LDFLAGS := -lstdc++ -lm -lpthread
endif

# CGo flags (passed via environment)
export CGO_CFLAGS  := -I$(abspath $(WHISPER_DIR)/include) -I$(abspath $(WHISPER_DIR)/ggml/include) -O2
export CGO_LDFLAGS := $(foreach lib,$(WHISPER_LIBS),$(abspath $(lib))) $(PLATFORM_LDFLAGS)

# ── Targets ──────────────────────────────────────────────

TEN_VAD_FRAMEWORK := third_party/ten-vad/lib/macOS/ten_vad.framework

RG_VERSION := 14.1.1
RG_ARM64_URL := https://github.com/BurntSushi/ripgrep/releases/download/$(RG_VERSION)/ripgrep-$(RG_VERSION)-aarch64-apple-darwin.tar.gz
RG_AMD64_URL := https://github.com/BurntSushi/ripgrep/releases/download/$(RG_VERSION)/ripgrep-$(RG_VERSION)-x86_64-apple-darwin.tar.gz

pkg/search/rg-darwin-arm64:
	@echo "Downloading ripgrep $(RG_VERSION) for darwin/arm64..."
	@tmpdir=$$(mktemp -d) && \
	  curl -fsSL $(RG_ARM64_URL) | tar -xz -C $$tmpdir && \
	  mv $$tmpdir/ripgrep-$(RG_VERSION)-aarch64-apple-darwin/rg $@ && \
	  rm -rf $$tmpdir
	@echo "Downloaded: $@"

pkg/search/rg-darwin-amd64:
	@echo "Downloading ripgrep $(RG_VERSION) for darwin/amd64..."
	@tmpdir=$$(mktemp -d) && \
	  curl -fsSL $(RG_AMD64_URL) | tar -xz -C $$tmpdir && \
	  mv $$tmpdir/ripgrep-$(RG_VERSION)-x86_64-apple-darwin/rg $@ && \
	  rm -rf $$tmpdir
	@echo "Downloaded: $@"

rg-download: pkg/search/rg-darwin-arm64 pkg/search/rg-darwin-amd64

build: whisper-lib rg-download
	go build -o tacit ./cmd/tacit/
ifeq ($(UNAME_S),Darwin)
	@echo "Bundling ten_vad.framework..."
	rm -rf ten_vad.framework
	cp -R $(TEN_VAD_FRAMEWORK) ten_vad.framework
	install_name_tool -rpath "$$(otool -l tacit | grep -A2 LC_RPATH | grep path | awk '{print $$2}')" @executable_path tacit
endif

test: whisper-lib
	go test ./...

whisper-lib: $(WHISPER_BUILD)/src/libwhisper.a

$(WHISPER_BUILD)/src/libwhisper.a:
	@command -v cmake >/dev/null 2>&1 || { echo "Error: cmake is required. Install with: brew install cmake"; exit 1; }
	@echo "Building whisper.cpp stacit libraries..."
	cmake -B $(WHISPER_BUILD) -S $(WHISPER_DIR) \
		-DBUILD_SHARED_LIBS=OFF \
		-DWHISPER_BUILD_EXAMPLES=OFF \
		-DWHISPER_BUILD_TESTS=OFF \
		-DWHISPER_BUILD_SERVER=OFF \
		-DCMAKE_BUILD_TYPE=Release
	cmake --build $(WHISPER_BUILD) --config Release -j

e2e-test: build
	./tacit process testdata/test_voice_recording.m4a
	go test -tags integration -v -count=1 ./pkg/process/ -run TestClassifier
ifeq ($(UNAME_S),Darwin)
	go test -tags "integration darwin" -v -count=1 -timeout 30s ./pkg/capture/ -run TestSpeaker_Stream_E2E
endif

# ── Mac app ──────────────────────────────────────────────
#
# Tacit.app is the menu-bar front end (cmd/tacit-app) with the CLI bundled
# beside it, which it runs as `tacit listen`:
#
#   Contents/MacOS/Tacit                  menu-bar app (pure Go + Wails)
#   Contents/Helpers/tacit                the CLI from `make build`
#   Contents/Frameworks/ten_vad.framework
#
# The CLI is in Helpers, not beside the app in MacOS: the default macOS
# filesystem is case-insensitive, so MacOS/tacit would overwrite MacOS/Tacit.
# The framework lives in Frameworks, where codesign expects nested code, so the
# bundled CLI gets an extra rpath to find it there.
#
# Signing is ad-hoc by default. macOS keys microphone and Screen Recording
# grants to an ad-hoc build's exact hash, so every rebuild asks again; sign with
# a stable identity (e.g. a self-signed "Code Signing" certificate made in
# Keychain Access) to keep grants across rebuilds:
#
#   make app SIGN_IDENTITY="Tacit Dev"

APP := build/Tacit.app
SIGN_IDENTITY ?= -
MACOS_MIN := 13.0

# CGO_* are overridden for the app: the whisper link flags exported above are the
# CLI's, and the app links none of it.
app: build
	rm -rf $(APP)
	mkdir -p $(APP)/Contents/MacOS $(APP)/Contents/Helpers $(APP)/Contents/Frameworks
	CGO_CFLAGS="-mmacosx-version-min=$(MACOS_MIN)" CGO_LDFLAGS="-mmacosx-version-min=$(MACOS_MIN)" \
		go build -o $(APP)/Contents/MacOS/Tacit ./cmd/tacit-app/
	cp tacit $(APP)/Contents/Helpers/tacit
	install_name_tool -add_rpath @executable_path/../Frameworks $(APP)/Contents/Helpers/tacit
	cp -R ten_vad.framework $(APP)/Contents/Frameworks/
	cp cmd/tacit-app/Info.plist $(APP)/Contents/Info.plist
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
	rm -f pkg/search/rg-darwin-arm64 pkg/search/rg-darwin-amd64
	go clean
