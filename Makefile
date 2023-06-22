all: angular-to-http check

SRC_FILES := $(filter-out $(wildcard src/ath/*_test.go), $(wildcard src/ath/*.go))

angular-to-http: *.go $(SRC_FILES)
	go build

clean:
	rm -f angular-to-http

check:
	make -C src/ath check

check-race:
	make -C src/ath check-race

.PHONY: check clean
