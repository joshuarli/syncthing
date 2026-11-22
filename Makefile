.PHONY: test clean

test:
	go run build.go test

clean:
	@if test -L ./syncthing || { test -e ./syncthing && ! test -f ./syncthing; }; then \
	  printf '%s\n' 'make clean: ./syncthing must be a local build file' >&2; exit 1; \
	fi
	rm -f -- ./syncthing
