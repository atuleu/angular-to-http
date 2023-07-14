all: angular-to-http check

angular-to-http:
	make -C cmd/angular-to-http

clean:
	make -C cmd/angular-to-http clean
	make -C internal/ath clean

check:
	make -C internal/ath check

check-race:
	make -C internal/ath check-race

.PHONY: check check-race clean angular-to-http
