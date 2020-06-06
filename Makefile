BUILD := go build
BUILD_ARMV6 := GOOS=linux GOARCH=arm GOARM=6 $(BUILD)
BUILD_ARMV7 := GOOS=linux GOARCH=arm GOARM=7 $(BUILD)

OUT_DIR := out

all: cmd

.PHONY: cmd
cmd:
	$(BUILD) -o $(OUT_DIR)/sds011 ./cmd
	$(BUILD_ARMV7) -o $(OUT_DIR)/armv7/sds011 ./cmd
	$(BUILD_ARMV6) -o $(OUT_DIR)/armv6/sds011 ./cmd

clean:
	rm -rf $(OUT_DIR)
